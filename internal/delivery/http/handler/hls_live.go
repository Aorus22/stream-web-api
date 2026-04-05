package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	uc "stream-web-api/internal/domain/usecase"
)

type HLSLiveHandler struct {
	sessionMgr *uc.HLSSessionManager
}

func NewHLSLiveHandler(sessionMgr *uc.HLSSessionManager) *HLSLiveHandler {
	return &HLSLiveHandler{sessionMgr: sessionMgr}
}

type StartHLSRequest struct {
	InfoHash  string  `json:"infoHash" binding:"required"`
	FileIndex int     `json:"fileIndex"`
	StartTime float64 `json:"startTime"`
}

func (h *HLSLiveHandler) StartHLSStream(c *gin.Context) {
	var req StartHLSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	session, err := h.sessionMgr.CreateSession(context.Background(), uc.CreateSessionParams{
		InfoHash:  req.InfoHash,
		FileIndex: req.FileIndex,
		StartTime: req.StartTime,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start HLS session: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessionId":   session.ID,
		"playlistUrl": "/hls-live/" + session.ID + "/playlist.m3u8",
		"startTime":   session.StartTime,
	})
}

func (h *HLSLiveHandler) ServeHLSFile(c *gin.Context) {
	sessionID := c.Param("id")
	session, ok := h.sessionMgr.GetSession(sessionID)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	fileParam := c.Param("file")
	fileParam = strings.TrimPrefix(fileParam, "/")

	if strings.Contains(fileParam, "..") {
		c.Status(http.StatusBadRequest)
		return
	}

	switch {
	case fileParam == "playlist.m3u8":
		data, err := os.ReadFile(session.PlaylistPath)
		if err != nil {
			c.Header("Content-Type", "application/vnd.apple.mpegurl")
			c.Header("Cache-Control", "no-cache")
			c.String(http.StatusOK, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:4\n")
			return
		}
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "application/vnd.apple.mpegurl", data)

	case strings.HasPrefix(fileParam, "seg_") && strings.HasSuffix(fileParam, ".m4s"):
		segmentPath := filepath.Join(session.SegmentDir, fileParam)
		info, err := os.Stat(segmentPath)
		if err != nil || info.Size() == 0 {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Content-Type", "video/mp4")
		c.Header("Cache-Control", "public, max-age=3600")
		c.File(segmentPath)

	case fileParam == "init.mp4":
		initPath := filepath.Join(session.SegmentDir, "init.mp4")
		info, err := os.Stat(initPath)
		if err != nil || info.Size() == 0 {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Content-Type", "video/mp4")
		c.Header("Cache-Control", "public, max-age=86400")
		c.File(initPath)

	default:
		c.Status(http.StatusNotFound)
	}
}

func (h *HLSLiveHandler) StopHLSStream(c *gin.Context) {
	sessionID := c.Param("id")
	h.sessionMgr.StopSession(sessionID)
	c.JSON(http.StatusOK, gin.H{"message": "Session stopped"})
}
