package handler

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	jsExecutorUC "torrent-stream/internal/usecase/js_executor"
)

// JSExecutorHandler handles JavaScript execution requests
type JSExecutorHandler struct {
	service *jsExecutorUC.Service
}

// NewJSExecutorHandler creates a new JS executor handler
func NewJSExecutorHandler(service *jsExecutorUC.Service) *JSExecutorHandler {
	return &JSExecutorHandler{
		service: service,
	}
}

// HandleExecuteJS handles POST /api/js/execute
func (h *JSExecutorHandler) HandleExecuteJS(c *gin.Context) {
	var req struct {
		Code     string `json:"code"`
		URL      string `json:"url"`
		PageType string `json:"pageType"`
		IsBase64 bool   `json:"isBase64"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	code := req.Code
	// Decode base64 if flag is set
	if req.IsBase64 && code != "" {
		decoded, err := base64.StdEncoding.DecodeString(code)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to decode base64 code: %v", err)})
			return
		}
		code = string(decoded)
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code is required"})
		return
	}

	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	// Default pageType to "list" if not provided
	pageType := req.PageType
	if pageType == "" {
		pageType = "list"
	}

	result, err := h.service.Execute(c.Request.Context(), code, req.URL, pageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandlePreviewHTML handles POST /api/js/preview
// Fetches HTML from the given URL
func (h *JSExecutorHandler) HandlePreviewHTML(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	// Fetch the HTML
	resp, err := http.Get(req.URL)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch URL: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.String(http.StatusBadGateway, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status))
		return
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to read response: %v", err))
		return
	}

	// Return the HTML as plain text
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, string(body))
}
