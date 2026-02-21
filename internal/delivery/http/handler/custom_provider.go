package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"torrent-stream/internal/usecase/custom_provider"
)

// CustomProviderHandler handles custom provider HTTP requests
type CustomProviderHandler struct {
	uc *custom_provider.CustomProviderUsecase
}

// NewCustomProviderHandler creates a new custom provider handler
func NewCustomProviderHandler(uc *custom_provider.CustomProviderUsecase) *CustomProviderHandler {
	return &CustomProviderHandler{uc: uc}
}

// CreateRequest represents the request to create/update a custom provider
type CreateRequest struct {
	Name     string `json:"name"`
	BaseURL  string `json:"baseUrl"`
	PageType string `json:"pageType"`
	Code     string `json:"code"`
	Language string `json:"language"`
}

// HandleCreate handles POST /api/custom-providers
func (h *CustomProviderHandler) HandleCreate(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	language := req.Language
	if language == "" {
		language = "javascript"
	}

	provider, err := h.uc.Create(req.Name, req.BaseURL, req.PageType, req.Code, language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, provider)
}

// HandleGetAll handles GET /api/custom-providers
func (h *CustomProviderHandler) HandleGetAll(c *gin.Context) {
	providers, err := h.uc.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, providers)
}

// HandleGetByID handles GET /api/custom-providers/:id
func (h *CustomProviderHandler) HandleGetByID(c *gin.Context) {
	id := c.Param("id")

	provider, err := h.uc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// HandleUpdate handles PUT /api/custom-providers/:id
func (h *CustomProviderHandler) HandleUpdate(c *gin.Context) {
	id := c.Param("id")

	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	language := req.Language
	if language == "" {
		language = "javascript"
	}

	provider, err := h.uc.Update(id, req.Name, req.BaseURL, req.PageType, req.Code, language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// HandleDelete handles DELETE /api/custom-providers/:id
func (h *CustomProviderHandler) HandleDelete(c *gin.Context) {
	id := c.Param("id")

	if err := h.uc.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}
