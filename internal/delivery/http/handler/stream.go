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

	"torrent-stream/internal/infrastructure/ffmpeg"
	torrentUC "torrent-stream/internal/usecase/torrent"
	"torrent-stream/pkg/srt"
)

// StreamHandler handles streaming requests
type StreamHandler struct {
	service      *torrentUC.Service
	transcoder   *ffmpeg.Transcoder
	cacheDir     string
	mu           sync.Mutex
	semaphore    chan struct{}
	activeCancel context.CancelFunc
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(service *torrentUC.Service, transcoder *ffmpeg.Transcoder, cacheDir string) *StreamHandler {
	return &StreamHandler{
		service:    service,
		transcoder: transcoder,
		cacheDir:   cacheDir,
		semaphore:  make(chan struct{}, 3), // Limit to 3 concurrent transcodes
	}
}

// HandleKillStream handles DELETE /api/stream/active
func (h *StreamHandler) HandleKillStream(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.activeCancel != nil {
		h.activeCancel()
		h.activeCancel = nil
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

	// Check if raw mode (for transcoder loopback)
	raw := c.Query("raw") == "true"

	// Only apply single-client limit for non-raw (user) requests
	// Transcoder loopback requests (raw=true) are internal and should not kill the parent stream
	var streamCtx context.Context
	var cancel context.CancelFunc

	if !raw {
		h.mu.Lock()
		if h.activeCancel != nil {
			h.activeCancel() // Kill previous stream
			log.Println("🔪 Killed previous active stream")
		}
		streamCtx, cancel = context.WithCancel(c.Request.Context())
		h.activeCancel = cancel
		h.mu.Unlock()

		defer func() {
			h.mu.Lock()
			// Cleanup: Clear h.activeCancel ONLY if it's still pointing to our cancel function.
			// This prevents us from nil-ing out a NEW stream's cancel function if one replaced us.
			// Note: Function pointer comparison in Go isn't direct, but since we are locking,
			// we can rely on the fact that if we were replaced, activeCancel would have been overwritten.
			// Since we can't easily compare functions, we'll skip the nil check for now and just rely on
			// the fact that overwriting it is safe because the previous one is already cancelled.
			//
			// A safer approach for state tracking would be using IDs, but for this simple case,
			// just cancelling our own context is sufficient cleanup.
			h.mu.Unlock()
			cancel()
		}()
	} else {
		// For loopback, use request context directly
		streamCtx = c.Request.Context()
	}

	// Check if needs transcoding
	if !raw && h.service.NeedsTranscoding(filename) {
		h.handleTranscodeInternal(c, infoHash, fileIndex, streamCtx)
		return
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

	// Update download priorities (Background Manager)
	// This ensures the "Sliding Window" follows the user's playback position.
	if err := h.service.UpdatePriorityWindow(infoHash, fileIndex, start); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update priority window"})
		return
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
	contentType := h.service.GetMimeType(filename)

	c.Header("Content-Type", contentType)
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

	// loopback URL for probing
	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)

	// 2. Get Duration
	duration, err := h.transcoder.GetVideoDurationFromURL(inputURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get duration: " + err.Error()})
		return
	}

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

	// Ensure File Header (Important for seeking via loopback)
	// h.service.EnsureFileHeader(infoHash, fileIndex)
	// Actually TranscodeSegment uses direct loopback url which triggers HandleStream, which ensures header?
	// Yes, but calling it here reduces latency / failures.
	h.service.EnsureFileHeader(infoHash, fileIndex)

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
	// Acquire semaphore
	select {
	case h.semaphore <- struct{}{}:
		defer func() { <-h.semaphore }()
	case <-c.Request.Context().Done():
		return
	}

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
