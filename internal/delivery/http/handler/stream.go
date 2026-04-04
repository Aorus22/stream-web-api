package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"stream-web-api/internal/domain/model"
	uc "stream-web-api/internal/domain/usecase"
	"stream-web-api/pkg/ranger"
)

const StreamSegmentDuration = 10.0

type StreamHandler struct {
	streamService  *uc.StreamUsecase
	torrentService *uc.TorrentUsecase
	directService  *uc.DirectDownloadUsecase
	cacheDir       string
}

func NewStreamHandler(
	streamService *uc.StreamUsecase,
	torrentService *uc.TorrentUsecase,
	directService *uc.DirectDownloadUsecase,
	cacheDir string,
) *StreamHandler {
	return &StreamHandler{
		streamService:  streamService,
		torrentService: torrentService,
		directService:  directService,
		cacheDir:       cacheDir,
	}
}

func (h *StreamHandler) HandleKillStream(c *gin.Context) {
	if h.streamService.HandleKillStream() {
		c.JSON(http.StatusOK, gin.H{"message": "Stream killed"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active stream"})
	}
}

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
	contentType := h.torrentService.GetMimeType(filename)

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
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))

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

func (h *StreamHandler) handleOnDemandDirectProxy(c *gin.Context, download *model.DirectDownload) {
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
	start, end, hasRange := ranger.ParseByteRange(rangeHeader)
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
		contentType = h.torrentService.GetMimeType(download.Filename)
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
			if s, e, t, ok := ranger.ParseContentRange(cr); ok {
				writeStart = s
				if t > 0 {
					total = t
				}
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

	contentType := h.torrentService.GetMimeType(filename)
	contentLength := end - start + 1

	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
	c.Status(http.StatusPartialContent)

	_, _ = f.Seek(start, io.SeekStart)
	_, _ = io.CopyN(c.Writer, f, contentLength)
}

func (h *StreamHandler) HandleStream(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	raw := c.Query("raw") == "true"
	download := c.Query("download") == "true"
	startTime, _ := strconv.ParseFloat(c.Query("t"), 0)

	result, reader, err := h.streamService.StreamTorrentFile(c.Request.Context(), c.Writer, infoHash, fileIndex, c.GetHeader("Range"), c.Query("d"), startTime, raw, download)
	if err != nil {
		switch err.Error() {
		case "torrent_not_ready":
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Torrent metadata not ready"})
		case "invalid_file_index":
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		case "torrent_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Torrent not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	if reader == nil {
		return
	}

	c.Header("Content-Type", result.ContentType)
	if download {
		name := filepath.Base(result.Filename)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", name))
	}
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", strconv.FormatInt(result.ContentLength, 10))

	if result.IsRangeRequest {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", result.ContentStart, result.ContentEnd, result.FileSize))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}

	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	h.copyWithTimeout(c.Writer, reader, result.ContentLength, c.Request.Context())
}

func (h *StreamHandler) HandleTranscode(c *gin.Context) {
	h.HandleStream(c)
}

func (h *StreamHandler) HandleDuration(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	duration, err := h.streamService.GetDuration(c.Request.Context(), infoHash, fileIndex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"duration": duration})
}

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

	c.Header("Content-Type", "application/json; charset=utf-8")
	err = h.streamService.ExtractSubtitle(c.Request.Context(), c.Writer, infoHash, fileIndex, streamIndex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func (h *StreamHandler) HandleReencode(c *gin.Context) {
	var req struct {
		InfoHash   string `json:"infoHash"`
		FileIndex  int    `json:"fileIndex"`
		DownloadID int    `json:"downloadId"`
		Resolution string `json:"resolution"`
		Bitrate    string `json:"bitrate"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	result, err := h.streamService.HandleReencode(c.Request.Context(), req.InfoHash, req.FileIndex, req.DownloadID, req.Resolution, req.Bitrate)
	if err != nil {
		switch err.Error() {
		case "Transcoder not available (FFmpeg not found)":
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		case "torrent_not_found", "download_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case "infoHash or downloadId required":
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *StreamHandler) HandleCancelReencode(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.streamService.HandleCancelReencode(req.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reencode job cancel requested"})
}

func (h *StreamHandler) HandleMediaInfo(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	info, err := h.streamService.GetMediaInfo(c.Request.Context(), infoHash, fileIndex)
	if err != nil {
		if err.Error() == "FFprobe not available" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"duration":  info.Duration,
		"subtitles": info.Subtitles,
	})
}

func (h *StreamHandler) HandleHLSMasterPlaylist(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	playlist, err := h.streamService.GetHLSPlaylist(c.Request.Context(), infoHash, fileIndex)
	if err != nil {
		if err.Error() == "Transcoder not available" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.String(http.StatusOK, playlist.Playlist)
}

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

	segmentIdx, err := strconv.Atoi(segmentStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid segment index"})
		return
	}

	if h.streamService.TranscoderNotAvailable() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Transcoder not available"})
		return
	}

	startTime := float64(segmentIdx) * StreamSegmentDuration

	duration, _ := h.streamService.GetCachedDuration(infoHash, fileIndex)
	h.streamService.UpdatePlayback(infoHash, fileIndex, startTime, duration, segmentIdx)

	h.streamService.EnsureFileHeader(infoHash, fileIndex)

	cacheSubDir := filepath.Join(h.cacheDir, infoHash, fmt.Sprintf("file_%d", fileIndex))
	if err := os.MkdirAll(cacheSubDir, 0755); err != nil {
		log.Printf("Failed to create cache dir: %v", err)
	}

	cachePath := filepath.Join(cacheSubDir, fmt.Sprintf("segment_%d.ts", segmentIdx))

	if info, err := os.Stat(cachePath); err == nil {
		if info.Size() > 1024 {
			log.Printf("📦 Serving cached segment: %s", cachePath)
			c.Header("Content-Type", "video/mp2t")
			c.File(cachePath)
			return
		}
		log.Printf("⚠️ Found invalid cache file (too small), removing: %s", cachePath)
		os.Remove(cachePath)
	}

	inputURL := h.streamService.GetLoopbackURL(infoHash, fileIndex)

	log.Printf("🎯 Segment %d: transcoding (time: %.1fs)", segmentIdx, startTime)

	cacheFile, err := os.Create(cachePath)
	if err != nil {
		log.Printf("Failed to create cache file: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Header("Content-Type", "video/mp2t")

	multiWriter := io.MultiWriter(c.Writer, cacheFile)

	ctx := c.Request.Context()

	err = h.streamService.TranscodeHLSSegment(ctx, multiWriter, inputURL, startTime, StreamSegmentDuration, cachePath)

	cacheFile.Close()

	if err != nil {
		log.Printf("❌ Segment transcode failed: %v", err)
		os.Remove(cachePath)
		return
	}
}

func (h *StreamHandler) copyWithTimeout(w io.Writer, r io.Reader, length int64, ctx context.Context) {
	buf := make([]byte, 64*1024)
	written := int64(0)
	lastProgress := time.Now()

	flusher, canFlush := w.(http.Flusher)

	for written < length {
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

		if n == 0 && err == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
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
