package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/manusia/user-service/internal/model"
	"github.com/manusia/user-service/internal/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) ListAll(c *gin.Context) {
	search := c.Query("search")
	var available *bool
	if av := c.Query("available"); av != "" {
		b := av == "true"
		available = &b
	}
	profiles, err := h.svc.ListAll(c.Request.Context(), search, available)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if profiles == nil {
		profiles = []model.UserProfile{}
	}
	c.JSON(http.StatusOK, profiles)
}

func (h *UserHandler) ListPage(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	if size <= 0 {
		size = 10
	}
	search := c.Query("search")
	var available *bool
	if av := c.Query("available"); av == "true" {
		b := true
		available = &b
	}
	result, err := h.svc.ListPage(c.Request.Context(), page, size, search, available)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if result.Content == nil {
		result.Content = []model.UserProfile{}
	}
	c.JSON(http.StatusOK, result)
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	p, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *UserHandler) GetByAuthID(c *gin.Context) {
	authID := c.Param("authId")
	p, err := h.svc.GetByAuthID(c.Request.Context(), authID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *UserHandler) Create(c *gin.Context) {
	var req model.CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *UserHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *UserHandler) UploadPhoto(c *gin.Context) {
	id := c.Param("id")
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file tidak ditemukan"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal baca file"})
		return
	}

	// Wrap bytes as reader
	p, err := h.svc.UploadPhotoBytes(c.Request.Context(), id, data, header.Filename, header.Header.Get("Content-Type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

// UpdateRating is called internally (e.g., from review-service via HTTP or Kafka consumer)
func (h *UserHandler) UpdateRating(c *gin.Context) {
	authID := c.Param("authId")
	var body struct {
		Rating       float64 `json:"rating"       binding:"required"`
		TotalReviews int     `json:"totalReviews" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.UpdateRating(c.Request.Context(), authID, body.Rating, body.TotalReviews); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
