package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/gin-gonic/gin"

	"torrent-stream/internal/domain"
	"torrent-stream/internal/infrastructure/ffmpeg"
	directUC "torrent-stream/internal/usecase/direct"
	torrentUC "torrent-stream/internal/usecase/torrent"
	"torrent-stream/pkg/srt"
)

// StreamHandler handles streaming requests.
// Uses the shared Transcoder (with built-in FFmpeg pool) and
// TorrentService (with built-in single-stream management) for all operations.
type StreamHandler struct {
	service       *torrentUC.Service
	directService *directUC.Service
	transcoder    *ffmpeg.Transcoder
	cacheDir      string

	// cachedDurations stores known video durations keyed by "infoHash/fileIndex".
	// This is populated lazily when HandleHLSMasterPlaylist probes duration.
	durationMu      sync.RWMutex
	cachedDurations map[string]float64
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(service *torrentUC.Service, directService *directUC.Service, transcoder *ffmpeg.Transcoder, cacheDir string) *StreamHandler {
	h := &StreamHandler{
		service:         service,
		directService:   directService,
		transcoder:      transcoder,
		cacheDir:        cacheDir,
		cachedDurations: make(map[string]float64),
	}

	// Register seek callback: when user seeks, pre-generate HLS segments ahead
	service.OnSeek(func(infoHash string, fileIndex int, segmentIdx int, timestamp float64) {
		h.prefetchSegments(infoHash, fileIndex, segmentIdx)
	})

	return h
}

// HandleDirectStream handles GET /stream/direct/:id
func (h *StreamHandler) HandleDirectStream(c *gin.Context) {
	if h.directService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Direct downloads not available"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}

	download, err := h.directService.GetDownload(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Download not found"})
		return
	}

	if download.Status == "on_demand" {
		h.handleOnDemandDirectProxy(c, download)
		return
	}

	if download.Status != "completed" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Download not complete"})
		return
	}

	filePath := download.FilePath
	if filePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	f, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stat file"})
		return
	}

	fileSize := info.Size()
	filename := info.Name()
	contentType := h.service.GetMimeType(filename)

	if c.Query("download") == "true" {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	}

	rangeHeader := c.GetHeader("Range")
	var start, end int64 = 0, fileSize - 1
	isRange := false

	if rangeHeader != "" {
		isRange = true
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
		if err != nil {
			start = 0
		}
		if strings.Contains(rangeHeader, "-") {
			parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
			if len(parts) == 2 && parts[1] != "" {
				end, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		}
		if end > fileSize-1 {
			end = fileSize - 1
		}
		if start < 0 {
			start = 0
		}
		if start > end {
			start = 0
			end = fileSize - 1
			isRange = false
		}
	}

	contentLength := end - start + 1

	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))

	if isRange {
		c.Status(http.StatusPartialContent)
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	} else {
		c.Status(http.StatusOK)
	}

	if _, err := f.Seek(start, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to seek"})
		return
	}

	_, _ = io.CopyN(c.Writer, f, contentLength)
}

func (h *StreamHandler) handleOnDemandDirectProxy(c *gin.Context, download *domain.DirectDownload) {
	if download.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing url"})
		return
	}
	if download.FilePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Missing cache file path"})
		return
	}

	h.directService.StartBackgroundPrefetch(download.ID)

	rangeHeader := c.GetHeader("Range")
	start, end, hasRange := parseByteRange(rangeHeader)
	// Serve from cache when we have a bounded range and it's already cached.
	if hasRange && end >= start && download.TotalBytes > 0 && h.directService.OnDemandIsCached(download.ID, start, end+1) {
		h.serveLocalRange(c, download.FilePath, download.Filename, start, end, download.TotalBytes)
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, download.URL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid url"})
		return
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch source"})
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = h.service.GetMimeType(download.Filename)
	}

	if resp.Header.Get("Accept-Ranges") != "" {
		c.Header("Accept-Ranges", resp.Header.Get("Accept-Ranges"))
	} else {
		c.Header("Accept-Ranges", "bytes")
	}
	if resp.Header.Get("Content-Range") != "" {
		c.Header("Content-Range", resp.Header.Get("Content-Range"))
	}
	if resp.Header.Get("Content-Length") != "" {
		c.Header("Content-Length", resp.Header.Get("Content-Length"))
	}
	c.Header("Content-Type", contentType)
	c.Status(resp.StatusCode)

	var (
		writeStart int64 = 0
		total      int64 = download.TotalBytes
	)
	if resp.StatusCode == http.StatusPartialContent {
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			if s, e, t, ok := parseContentRange(cr); ok {
				writeStart = s
				if t > 0 {
					total = t
				}
				// If server returned a bounded end, prefer it for cache record.
				if !hasRange {
					start, end, hasRange = s, e, true
				}
			}
		}
	} else if resp.StatusCode == http.StatusOK {
		writeStart = 0
		if resp.ContentLength > 0 {
			total = resp.ContentLength
		}
	}

	release := h.directService.OnDemandAcquireFileLock(download.ID)
	defer release()

	cacheFile, err := os.OpenFile(download.FilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		// Can't cache, but still proxy to client.
		_, _ = io.Copy(c.Writer, resp.Body)
		return
	}
	defer cacheFile.Close()

	buf := make([]byte, 1024*256)
	var written int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, _ = c.Writer.Write(buf[:n])
			_, _ = cacheFile.WriteAt(buf[:n], writeStart+written)
			written += int64(n)
		}
		if readErr != nil {
			break
		}
	}

	if written > 0 {
		h.directService.OnDemandRecordRange(download.ID, writeStart, writeStart+written, total, contentType)
	}
}

func (h *StreamHandler) serveLocalRange(c *gin.Context, filePath string, filename string, start int64, end int64, total int64) {
	f, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	defer f.Close()

	contentType := h.service.GetMimeType(filename)
	contentLength := end - start + 1

	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
	c.Status(http.StatusPartialContent)

	_, _ = f.Seek(start, io.SeekStart)
	_, _ = io.CopyN(c.Writer, f, contentLength)
}

func parseByteRange(rangeHeader string) (start int64, end int64, ok bool) {
	// Supports "bytes=start-end" where end is optional.
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, -1, false
	}
	parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
	if len(parts) != 2 {
		return 0, -1, false
	}
	s, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || s < 0 {
		return 0, -1, false
	}
	e := int64(-1)
	if parts[1] != "" {
		if parsed, err := strconv.ParseInt(parts[1], 10, 64); err == nil && parsed >= s {
			e = parsed
		}
	}
	return s, e, true
}

func parseContentRange(cr string) (start int64, end int64, total int64, ok bool) {
	// Expected "bytes start-end/total"
	if !strings.HasPrefix(cr, "bytes ") {
		return 0, 0, 0, false
	}
	cr = strings.TrimPrefix(cr, "bytes ")
	parts := strings.Split(cr, "/")
	if len(parts) != 2 {
		return 0, 0, 0, false
	}
	rangePart := parts[0]
	totalPart := parts[1]

	re := strings.Split(rangePart, "-")
	if len(re) != 2 {
		return 0, 0, 0, false
	}
	s, err := strconv.ParseInt(strings.TrimSpace(re[0]), 10, 64)
	if err != nil {
		return 0, 0, 0, false
	}
	e, err := strconv.ParseInt(strings.TrimSpace(re[1]), 10, 64)
	if err != nil {
		return 0, 0, 0, false
	}

	t := int64(0)
	if totalPart != "*" {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(totalPart), 10, 64); err == nil {
			t = parsed
		}
	}
	return s, e, t, true
}

// HandleKillStream handles DELETE /api/stream/active
func (h *StreamHandler) HandleKillStream(c *gin.Context) {
	if h.service.KillActiveStream() {
		c.JSON(http.StatusOK, gin.H{"message": "Stream killed"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active stream"})
	}
}

// HandleStream handles GET /stream/:infoHash/:fileIndex
func (h *StreamHandler) HandleStream(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	// Get torrent
	t := h.service.GetTorrent(infoHash)
	if t == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Torrent not found"})
		return
	}

	torrentHandle, ok := t.(*torrent.Torrent)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid torrent handle"})
		return
	}

	if torrentHandle.Info() == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Torrent metadata not ready"})
		return
	}

	files := torrentHandle.Files()
	if fileIndex < 0 || fileIndex >= len(files) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	file := files[fileIndex]
	fileSize := file.Length()
	filename := file.DisplayPath()

	// Check if raw mode (for transcoder loopback) or download mode
	download := c.Query("download") == "true"
	raw := c.Query("raw") == "true" || download

	contentType := h.service.GetMimeType(filename)
	// For non-raw video requests, serve as fMP4 via TranscodeStream (remux: copy video, re-encode audio)
	if !raw && strings.HasPrefix(contentType, "video/") {
		// AcquireStream kills any previous active stream and returns a new cancellable context
		description := fmt.Sprintf("stream %s/%d", infoHash, fileIndex)
		streamCtx, cancel := h.service.AcquireStream(c.Request.Context(), description)
		defer cancel()

		h.handleTranscodeInternal(c, infoHash, fileIndex, streamCtx)
		return
	}

	// Below this point: either raw=true (loopback for FFmpeg) or non-video files
	// For loopback, use request context directly (no stream management)
	streamCtx := c.Request.Context()

	if !raw {
		// Non-video file (audio, etc.) — apply single-client stream management
		description := fmt.Sprintf("stream %s/%d", infoHash, fileIndex)
		var cancel context.CancelFunc
		streamCtx, cancel = h.service.AcquireStream(c.Request.Context(), description)
		defer cancel()

		// Check if needs transcoding (audio files with incompatible codecs)
		if h.service.NeedsTranscoding(filename) {
			h.handleTranscodeInternal(c, infoHash, fileIndex, streamCtx)
			return
		}
	}

	// Parse range header
	rangeHeader := c.GetHeader("Range")
	var start, end int64 = 0, fileSize - 1

	if rangeHeader != "" {
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
		if err != nil {
			start = 0
		}

		// Check for end value
		if strings.Contains(rangeHeader, "-") {
			parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
			if len(parts) == 2 && parts[1] != "" {
				end, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		}

		if end > fileSize-1 {
			end = fileSize - 1
		}
	}

	// Get reader
	reader, err := h.service.GetFileReader(infoHash, fileIndex, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file reader"})
		return
	}

	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	// Set headers
	contentLength := end - start + 1
	contentType = h.service.GetMimeType(filename)

	c.Header("Content-Type", contentType)
	if download {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filename)))
	}
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))

	if rangeHeader != "" {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}

	log.Printf("📡 Streaming %s [%d-%d] (%d bytes)", filename, start, end, contentLength)

	// Stream with timeout handling
	h.copyWithTimeout(c.Writer, reader, contentLength, streamCtx)
}

// handleTranscodeInternal handles transcoding internally
func (h *StreamHandler) handleTranscodeInternal(c *gin.Context, infoHash string, fileIndex int, ctx context.Context) {
	if h.transcoder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Transcoding not available (FFmpeg not found)"})
		return
	}

	t := h.service.GetTorrent(infoHash)
	if t == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Torrent not found"})
		return
	}

	torrentHandle, ok := t.(*torrent.Torrent)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid torrent handle"})
		return
	}

	files := torrentHandle.Files()
	if fileIndex < 0 || fileIndex >= len(files) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	file := files[fileIndex]

	// Get start time from query param
	startTime := float64(0)
	if t := c.Query("t"); t != "" {
		startTime, _ = strconv.ParseFloat(t, 64)
	}

	// Start background sequential download (helps direct mode buffer smoothly).
	// If the client is seeking, move the download pointer near the seek time instead.
	if startTime <= 0 {
		if err := h.service.StartFileDownload(infoHash, fileIndex); err != nil {
			log.Printf("⚠️ Failed to start file download: %v", err)
		}
	} else {
		// Duration is required to translate timestamp -> piece index accurately.
		// It should be populated by /api/metadata (HandleMediaInfo) or HLS playlist probing.
		if d, ok := h.getCachedDuration(infoHash, fileIndex); ok && d > 0 {
			if err := h.service.SeekFileDownload(infoHash, fileIndex, startTime, d); err != nil {
				log.Printf("⚠️ Failed to seek file download: %v", err)
			}
		} else if qd := c.Query("d"); qd != "" {
			if parsed, err := strconv.ParseFloat(qd, 64); err == nil && parsed > 0 {
				h.setCachedDuration(infoHash, fileIndex, parsed)
				if err := h.service.SeekFileDownload(infoHash, fileIndex, startTime, parsed); err != nil {
					log.Printf("⚠️ Failed to seek file download: %v", err)
				}
			}
		}
	}

	// Ensure header is ready before starting FFmpeg to prevent probe failure
	h.service.EnsureFileHeader(infoHash, fileIndex)

	pieceLength := torrentHandle.Info().PieceLength
	startPiece := int(file.Offset() / pieceLength)

	// Wait for first piece (header) only
	if err := h.service.WaitForPieces(infoHash, startPiece, startPiece, 10*time.Second); err != nil {
		log.Printf("⚠️ Warning: Timeout waiting for header, starting FFmpeg anyway: %v", err)
	}

	// Construct loopback URL
	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)

	log.Printf("🎬 Transcoding %s from %.0f seconds using loopback", file.DisplayPath(), startTime)

	// Use the cancellable context here!
	transcodeErr := h.transcoder.TranscodeStream(ctx, c.Writer, inputURL, file.Length(), file.DisplayPath(), startTime)
	if transcodeErr != nil {
		// Only log error if not cancelled
		if ctx.Err() != context.Canceled {
			log.Printf("❌ Transcode error: %v", transcodeErr)
		} else {
			log.Printf("🛑 Transcode stopped (user disconnected)")
		}
	}
}

// HandleTranscode handles GET /transcode/:infoHash/:fileIndex
func (h *StreamHandler) HandleTranscode(c *gin.Context) {
	// infoHash & fileIndex not needed directly as we delegate to HandleStream
	h.HandleStream(c)
}

// HandleDuration handles GET /api/duration/:infoHash/:fileIndex
func (h *StreamHandler) HandleDuration(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	if h.transcoder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "FFprobe not available"})
		return
	}

	// Ensure header is ready for probing
	h.service.EnsureFileHeader(infoHash, fileIndex)

	// Wait for header (piece 0)
	// We calculate startPiece manually as we don't have direct access to file struct here easily without looking up again
	// But service.WaitForPieces needs piece index.
	// Let's assume piece 0 is startPiece for simplicity or use a helper in service.
	// Actually, just waiting a bit or relying on loopback blocking is mostly fine for duration.
	// But to be safe:
	// h.service.WaitForPieces(infoHash, 0, 0, 10*time.Second)
	// The problem is we don't know if Piece 0 is the start of THIS file.
	// But EnsureFileHeader handles priority correctly using file offset.
	// Let's just trust EnsureFileHeader + Loopback blocking.

	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)
	duration, err := h.transcoder.GetVideoDurationFromURL(inputURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get duration: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"duration": duration})
}

// HandleStreamSubtitle handles GET /api/stream/:infoHash/:fileIndex/sub/:streamIndex
func (h *StreamHandler) HandleStreamSubtitle(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}
	streamIndex, err := strconv.Atoi(c.Param("streamIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stream index"})
		return
	}

	if h.transcoder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Transcoding not available"})
		return
	}

	// Ensure file header is ready
	h.service.EnsureFileHeader(infoHash, fileIndex)

	// Smart wait for first piece (header/metadata)
	t := h.service.GetTorrent(infoHash)
	if t != nil {
		if torrentHandle, ok := t.(*torrent.Torrent); ok && torrentHandle.Info() != nil {
			files := torrentHandle.Files()
			if fileIndex >= 0 && fileIndex < len(files) {
				file := files[fileIndex]
				pieceLength := torrentHandle.Info().PieceLength
				startPiece := int(file.Offset() / pieceLength)
				// Wait for first piece
				h.service.WaitForPieces(infoHash, startPiece, startPiece, 10*time.Second)
			}
		}
	} else {
		time.Sleep(500 * time.Millisecond)
	}

	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)

	// Extract SRT
	var buf bytes.Buffer
	if err := h.transcoder.ExtractSubtitle(inputURL, streamIndex, &buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Parse to JSON cues
	// We use SRT extraction now, which matches srt.Parse expectation.
	cues := srt.Parse(buf.Bytes())

	log.Printf("Subtitle extraction: Stream %d, Bytes %d, Cues %d", streamIndex, buf.Len(), len(cues))

	c.JSON(http.StatusOK, cues)
}

// HandleReencode handles POST /api/reencode
func (h *StreamHandler) HandleReencode(c *gin.Context) {
	if h.transcoder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Transcoder not available (FFmpeg not found)"})
		return
	}

	var req struct {
		InfoHash   string `json:"infoHash"`
		FileIndex  int    `json:"fileIndex"`
		DownloadID int    `json:"downloadId"`
		Resolution string `json:"resolution"` // e.g., "1280x720"
		Bitrate    string `json:"bitrate"`    // e.g., "2000k"
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Resolution == "" {
		req.Resolution = "1280:720" // Default 720p (using colon for FFmpeg scale filter)
	} else {
		// Ensure it uses : instead of x for FFmpeg
		req.Resolution = strings.Replace(req.Resolution, "x", ":", 1)
	}

	if req.Bitrate == "" {
		req.Bitrate = "2000k"
	}

	var inputURL string
	var filename string
	var baseDir string

	if req.InfoHash != "" {
		// Torrent file
		t := h.service.GetTorrent(req.InfoHash)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Torrent not found"})
			return
		}

		torrentHandle, ok := t.(*torrent.Torrent)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid torrent handle"})
			return
		}

		files := torrentHandle.Files()
		if req.FileIndex < 0 || req.FileIndex >= len(files) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
			return
		}

		file := files[req.FileIndex]
		filename = file.DisplayPath()
		inputURL = fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), req.InfoHash, req.FileIndex)
		baseDir = filepath.Join(h.cacheDir, "exports", req.InfoHash)

		// Start download if not done
		h.service.StartFileDownload(req.InfoHash, req.FileIndex)
	} else if req.DownloadID != 0 {
		// Direct download
		dl, err := h.directService.GetDownload(req.DownloadID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Download not found"})
			return
		}

		filename = dl.Filename
		if dl.Status == "on_demand" {
			inputURL = dl.URL
		} else {
			inputURL = fmt.Sprintf("http://127.0.0.1:%d/stream/direct/%d?raw=true", h.service.GetPort(), req.DownloadID)
		}
		baseDir = filepath.Join(h.cacheDir, "exports", fmt.Sprintf("direct_%d", req.DownloadID))
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "infoHash or downloadId required"})
		return
	}

	// Create output path
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create export directory"})
		return
	}

	// Sanitize filename for output
	cleanName := filepath.Base(filename)
	ext := filepath.Ext(cleanName)
	nameWithoutExt := strings.TrimSuffix(cleanName, ext)
	outName := fmt.Sprintf("%s_%s.mp4", nameWithoutExt, strings.ReplaceAll(req.Resolution, ":", "p"))
	outputPath := filepath.Join(baseDir, outName)

	// Start reencoding in background
	go func() {
		log.Printf("🚀 Reencoding background job started for %s", filename)
		// Use a long timeout for background reencoding
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer cancel()

		err := h.transcoder.ReencodeToFile(ctx, inputURL, outputPath, req.Resolution, req.Bitrate)
		if err != nil {
			log.Printf("❌ Background reencode failed for %s: %v", filename, err)
			// Maybe clean up partial file
			os.Remove(outputPath)
		} else {
			log.Printf("✅ Background reencode complete: %s", outputPath)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message":    "Reencoding started in background",
		"outputPath": outputPath,
	})
}

// HandleMediaInfo handles GET /api/metadata/:infoHash/:fileIndex
func (h *StreamHandler) HandleMediaInfo(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	if h.transcoder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "FFprobe not available"})
		return
	}

	// Prioritize header
	h.service.EnsureFileHeader(infoHash, fileIndex)

	// Smart wait for first piece
	t := h.service.GetTorrent(infoHash)
	if t != nil {
		if torrentHandle, ok := t.(*torrent.Torrent); ok && torrentHandle.Info() != nil {
			files := torrentHandle.Files()
			if fileIndex >= 0 && fileIndex < len(files) {
				file := files[fileIndex]
				pieceLength := torrentHandle.Info().PieceLength
				startPiece := int(file.Offset() / pieceLength)
				h.service.WaitForPieces(infoHash, startPiece, startPiece, 10*time.Second)
			}
		}
	} else {
		time.Sleep(500 * time.Millisecond)
	}

	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)

	// Get Duration
	duration, err := h.transcoder.GetVideoDurationFromURL(inputURL)
	if err != nil {
		log.Printf("Metadata error (duration): %v", err)
	}
	if duration > 0 {
		h.setCachedDuration(infoHash, fileIndex, duration)
	}

	// Get Subtitles
	subs, err := h.transcoder.GetEmbeddedSubtitles(inputURL)
	if err != nil {
		log.Printf("Metadata error (subtitles): %v", err)
		subs = []ffmpeg.SubtitleStream{}
	}

	c.JSON(http.StatusOK, gin.H{
		"duration":  duration,
		"subtitles": subs,
	})
}

// copyWithTimeout streams data with timeout handling
func (h *StreamHandler) copyWithTimeout(w io.Writer, r io.Reader, length int64, ctx context.Context) {
	buf := make([]byte, 64*1024)
	written := int64(0)
	lastProgress := time.Now()

	flusher, canFlush := w.(http.Flusher)

	for written < length {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		maxRead := int64(len(buf))
		if length-written < maxRead {
			maxRead = length - written
		}

		n, err := r.Read(buf[:maxRead])

		// Prevent tight loop if Read returns 0 bytes and nil error
		if n == 0 && err == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				// Client disconnected
				return
			}
			written += int64(n)
			lastProgress = time.Now()

			if canFlush {
				flusher.Flush()
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}

			if time.Since(lastProgress) > 30*time.Second {
				log.Printf("Read timeout after 30 seconds")
				return
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
}

// HLS Constants
const SegmentDuration = 10.0

// --- Duration cache helpers ---

func (h *StreamHandler) durationKey(infoHash string, fileIndex int) string {
	return fmt.Sprintf("%s/%d", infoHash, fileIndex)
}

func (h *StreamHandler) getCachedDuration(infoHash string, fileIndex int) (float64, bool) {
	h.durationMu.RLock()
	defer h.durationMu.RUnlock()
	d, ok := h.cachedDurations[h.durationKey(infoHash, fileIndex)]
	return d, ok
}

func (h *StreamHandler) setCachedDuration(infoHash string, fileIndex int, duration float64) {
	h.durationMu.Lock()
	defer h.durationMu.Unlock()
	h.cachedDurations[h.durationKey(infoHash, fileIndex)] = duration
	// Also inform the playback state tracker so seek byte estimation is accurate
	h.service.UpdatePlaybackDuration(infoHash, fileIndex, duration)
}

// --- HLS pre-generation on seek ---

// prefetchSegments generates HLS segments ahead of the given segment index in the background.
// This is triggered by seek detection — when the user jumps far ahead, we start transcoding
// the next N segments immediately so they're cached when the player requests them.
func (h *StreamHandler) prefetchSegments(infoHash string, fileIndex int, fromSegment int) {
	if h.transcoder == nil {
		return
	}

	duration, hasDuration := h.getCachedDuration(infoHash, fileIndex)
	if !hasDuration {
		log.Printf("⚠️ Prefetch skipped: duration unknown for %s/%d", infoHash, fileIndex)
		return
	}

	totalSegments := int(math.Ceil(duration / SegmentDuration))

	// Pre-generate up to PrefetchSegments segments ahead (skip the one being requested now)
	for i := 1; i <= 3; i++ {
		segIdx := fromSegment + i
		if segIdx >= totalSegments {
			break
		}

		// Check if already cached
		cacheSubDir := filepath.Join(h.cacheDir, infoHash, fmt.Sprintf("file_%d", fileIndex))
		cachePath := filepath.Join(cacheSubDir, fmt.Sprintf("segment_%d.ts", segIdx))

		if info, err := os.Stat(cachePath); err == nil && info.Size() > 1024 {
			continue // Already cached, skip
		}

		go h.generateSegmentInBackground(infoHash, fileIndex, segIdx, cachePath)
	}
}

// generateSegmentInBackground transcodes a single segment to the cache file.
// Runs in a background goroutine — errors are logged, not returned.
func (h *StreamHandler) generateSegmentInBackground(infoHash string, fileIndex int, segmentIdx int, cachePath string) {
	startTime := float64(segmentIdx) * SegmentDuration

	log.Printf("🔮 Prefetching segment %d (time: %.1fs) for %s/%d", segmentIdx, startTime, infoHash, fileIndex)

	// Ensure cache dir exists
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("⚠️ Prefetch: failed to create cache dir: %v", err)
		return
	}

	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)

	// Ensure header pieces are ready
	h.service.EnsureFileHeader(infoHash, fileIndex)

	// Detect codecs
	videoCodec, audioCodec, err := h.transcoder.GetStreamDetails(inputURL)
	if err != nil {
		log.Printf("⚠️ Prefetch: codec detection failed: %v", err)
	}

	// Create cache file
	cacheFile, err := os.Create(cachePath)
	if err != nil {
		log.Printf("⚠️ Prefetch: failed to create cache file: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err = h.transcoder.TranscodeSegment(ctx, cacheFile, inputURL, startTime, SegmentDuration, videoCodec, audioCodec)
	cacheFile.Close()

	if err != nil {
		log.Printf("⚠️ Prefetch: segment %d transcode failed: %v", segmentIdx, err)
		os.Remove(cachePath)
		return
	}

	log.Printf("✅ Prefetched segment %d for %s/%d", segmentIdx, infoHash, fileIndex)
}

// HandleHLSMasterPlaylist handles GET /hls/:infoHash/:fileIndex/playlist.m3u8
func (h *StreamHandler) HandleHLSMasterPlaylist(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	if h.transcoder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Transcoder not available"})
		return
	}

	// 1. Ensure file header (for probing)
	h.service.EnsureFileHeader(infoHash, fileIndex)

	// Start downloading the entire file sequentially (background).
	// Anacrolix/torrent will download pieces in order: 0, 1, 2, ...
	if err := h.service.StartFileDownload(infoHash, fileIndex); err != nil {
		log.Printf("⚠️ Failed to start file download: %v", err)
	}

	// loopback URL for probing
	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)

	// 2. Get Duration
	duration, err := h.transcoder.GetVideoDurationFromURL(inputURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get duration: " + err.Error()})
		return
	}

	// Cache duration for use by HandleHLSSegment (playback tracking, piece calculation)
	h.setCachedDuration(infoHash, fileIndex, duration)
	// Also notify the service so playback state has the duration
	h.service.UpdatePlaybackDuration(infoHash, fileIndex, duration)

	// 3. Generate Playlist
	totalSegments := int(math.Ceil(duration / SegmentDuration))

	var playlist strings.Builder
	playlist.WriteString("#EXTM3U\n")
	playlist.WriteString("#EXT-X-VERSION:3\n")
	playlist.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", int(SegmentDuration)))
	playlist.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	playlist.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	for i := 0; i < totalSegments; i++ {
		segDur := SegmentDuration
		if i == totalSegments-1 {
			segDur = duration - (float64(i) * SegmentDuration)
		}
		playlist.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", segDur))
		playlist.WriteString(fmt.Sprintf("segment/segment_%d.ts\n", i))
	}

	playlist.WriteString("#EXT-X-ENDLIST\n")

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.String(http.StatusOK, playlist.String())
}

// HandleHLSSegment handles GET /hls/:infoHash/:fileIndex/segment/:segment.ts
func (h *StreamHandler) HandleHLSSegment(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	segmentStr := c.Param("segment")
	segmentStr = strings.TrimPrefix(segmentStr, "segment_")
	segmentStr = strings.TrimSuffix(segmentStr, ".ts")

	segmentIndex, err := strconv.Atoi(segmentStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid segment index"})
		return
	}

	if h.transcoder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Transcoder not available"})
		return
	}

	startTime := float64(segmentIndex) * SegmentDuration

	// Report playback position — triggers seek detection, download pointer move, and prefetch callbacks
	duration, _ := h.getCachedDuration(infoHash, fileIndex)
	h.service.UpdatePlayback(infoHash, fileIndex, startTime, duration, segmentIndex)

	// Ensure file header is available for FFmpeg probing
	h.service.EnsureFileHeader(infoHash, fileIndex)

	log.Printf("🎯 Segment %d: transcoding (time: %.1fs)", segmentIndex, startTime)

	// Cache Path
	cacheSubDir := filepath.Join(h.cacheDir, infoHash, fmt.Sprintf("file_%d", fileIndex))
	if err := os.MkdirAll(cacheSubDir, 0755); err != nil {
		log.Printf("Failed to create cache dir: %v", err)
	}

	cachePath := filepath.Join(cacheSubDir, fmt.Sprintf("segment_%d.ts", segmentIndex))

	// Check Cache
	if info, err := os.Stat(cachePath); err == nil {
		if info.Size() > 1024 { // Ensure file is at least 1KB
			log.Printf("📦 Serving cached segment: %s", cachePath)
			c.Header("Content-Type", "video/mp2t")
			c.File(cachePath)
			return
		}
		// Found invalid cache file, remove it
		log.Printf("⚠️ Found invalid cache file (too small), removing: %s", cachePath)
		os.Remove(cachePath)
	}

	// Transcode and Cache
	// Concurrency is managed by the FFmpeg pool (Transcoder.Acquire) — no handler-level semaphore needed.

	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)

	// Create Cache File
	cacheFile, err := os.Create(cachePath)
	if err != nil {
		log.Printf("Failed to create cache file: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	// Don't close immediately, wait for transcode

	c.Header("Content-Type", "video/mp2t")

	// MultiWriter: Write to Response AND Cache File
	multiWriter := io.MultiWriter(c.Writer, cacheFile)

	ctx := c.Request.Context()

	// Smart Codec: Get details first
	videoCodec, audioCodec, err := h.transcoder.GetStreamDetails(inputURL)
	if err != nil {
		log.Printf("⚠️ Failed to detect codecs: %v", err)
		// Proceed with default transcoding (empty codecs will trigger libx264/aac)
	}

	err = h.transcoder.TranscodeSegment(ctx, multiWriter, inputURL, startTime, SegmentDuration, videoCodec, audioCodec)

	// Close cache file after writing
	cacheFile.Close()

	if err != nil {
		log.Printf("❌ Segment transcode failed: %v", err)
		// Clean up partial file
		os.Remove(cachePath)
		// If headers already written (which they are), we can't send JSON error easily.
		// But Gin/HTTP might have already sent 200.
		return
	}
}
