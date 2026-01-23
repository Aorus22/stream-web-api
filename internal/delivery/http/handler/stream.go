package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/gin-gonic/gin"

	"torrent-stream/internal/infrastructure/ffmpeg"
	torrentUC "torrent-stream/internal/usecase/torrent"
)

// StreamHandler handles streaming requests
type StreamHandler struct {
	service    *torrentUC.Service
	transcoder *ffmpeg.Transcoder
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(service *torrentUC.Service, transcoder *ffmpeg.Transcoder) *StreamHandler {
	return &StreamHandler{
		service:    service,
		transcoder: transcoder,
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

	// Check if raw mode (for transcoder)
	raw := c.Query("raw") == "true"

	// Check if needs transcoding
	if !raw && h.service.NeedsTranscoding(filename) {
		h.handleTranscodeInternal(c, infoHash, fileIndex)
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

	// Prepare file for streaming
	if err := h.service.GetFileForStreaming(infoHash, fileIndex, start, end); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare file"})
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
	h.copyWithTimeout(c.Writer, reader, contentLength)
}

// handleTranscodeInternal handles transcoding internally
func (h *StreamHandler) handleTranscodeInternal(c *gin.Context, infoHash string, fileIndex int) {
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

	// Prioritize pieces first
	h.service.GetFileForStreaming(infoHash, fileIndex, 0, 10*1024*1024)

	// Construct loopback URL
	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)

	log.Printf("🎬 Transcoding %s from %.0f seconds using loopback", file.DisplayPath(), startTime)

	err := h.transcoder.TranscodeStream(c.Writer, c.Request, inputURL, file.Length(), file.DisplayPath(), startTime)
	if err != nil {
		log.Printf("❌ Transcode error: %v", err)
	}
}

// HandleTranscode handles GET /transcode/:infoHash/:fileIndex
func (h *StreamHandler) HandleTranscode(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	h.handleTranscodeInternal(c, infoHash, fileIndex)
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

	// Prioritize first pieces
	h.service.GetFileForStreaming(infoHash, fileIndex, 0, 5*1024*1024)

	time.Sleep(500 * time.Millisecond)

	// Get duration via loopback
	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", h.service.GetPort(), infoHash, fileIndex)
	duration, err := h.transcoder.GetVideoDurationFromURL(inputURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get duration: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"duration": duration})
}

// copyWithTimeout streams data with timeout handling
func (h *StreamHandler) copyWithTimeout(w io.Writer, r io.Reader, length int64) {
	buf := make([]byte, 64*1024)
	written := int64(0)
	lastProgress := time.Now()

	flusher, canFlush := w.(http.Flusher)

	for written < length {
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
				log.Printf("Client disconnected: %v", writeErr)
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
