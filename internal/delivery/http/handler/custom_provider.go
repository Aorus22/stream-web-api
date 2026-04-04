package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	uc "stream-web-api/internal/domain/usecase"
)

type CustomProviderHandler struct {
	service *uc.CustomProviderUsecase
}

func NewCustomProviderHandler(service *uc.CustomProviderUsecase) *CustomProviderHandler {
	return &CustomProviderHandler{service: service}
}

type CreateRequest struct {
	Name     string `json:"name"`
	BaseURL  string `json:"baseUrl"`
	PageType string `json:"pageType"`
	Code     string `json:"code"`
	Language string `json:"language"`
}

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

	provider, err := h.service.Create(req.Name, req.BaseURL, req.PageType, req.Code, language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, provider)
}

func (h *CustomProviderHandler) HandleGetAll(c *gin.Context) {
	providers, err := h.service.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, providers)
}

func (h *CustomProviderHandler) HandleGetByID(c *gin.Context) {
	id := c.Param("id")

	provider, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

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

	provider, err := h.service.Update(id, req.Name, req.BaseURL, req.PageType, req.Code, language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

func (h *CustomProviderHandler) HandleDelete(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}
