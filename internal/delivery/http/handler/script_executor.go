package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	uc "stream-web-api/internal/domain/usecase"
)

type ScriptExecutorHandler struct {
	service *uc.ScriptExecutorUsecase
}

func NewScriptExecutorHandler(service *uc.ScriptExecutorUsecase) *ScriptExecutorHandler {
	return &ScriptExecutorHandler{
		service: service,
	}
}

func (h *ScriptExecutorHandler) HandleExecuteScript(c *gin.Context) {
	var req struct {
		Code     string `json:"code"`
		URL      string `json:"url"`
		PageType string `json:"pageType"`
		IsBase64 bool   `json:"isBase64"`
		Language string `json:"language"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.service.ExecuteWithBase64(c.Request.Context(), req.Code, req.URL, req.PageType, req.Language, req.IsBase64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *ScriptExecutorHandler) HandlePreviewHTML(c *gin.Context) {
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

	content, err := h.service.FetchURLContent(req.URL)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, content)
}
