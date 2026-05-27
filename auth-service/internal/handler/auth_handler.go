package handler

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manusia/auth-service/internal/config"
	"github.com/manusia/auth-service/internal/model"
	"github.com/manusia/auth-service/internal/service"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type AuthHandler struct {
	svc  *service.AuthService
	cfg  *config.Config
	minio *minio.Client
}

func NewAuthHandler(svc *service.AuthService, cfg *config.Config) (*AuthHandler, error) {
	mc, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccess, cfg.MinioSecret, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	exists, err := mc.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := mc.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
		// Set public-read policy
		policy := fmt.Sprintf(`{
			"Version":"2012-10-17",
			"Statement":[{
				"Effect":"Allow",
				"Principal":"*",
				"Action":["s3:GetObject"],
				"Resource":["arn:aws:s3:::%s/*"]
			}]
		}`, cfg.MinioBucket)
		if err := mc.SetBucketPolicy(ctx, cfg.MinioBucket, policy); err != nil {
			return nil, err
		}
	}

	return &AuthHandler{svc: svc, cfg: cfg, minio: mc}, nil
}

func (h *AuthHandler) Register(c *gin.Context) {
	// Parse multipart form (max 10 MB)
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request tidak valid: " + err.Error()})
		return
	}

	req := model.RegisterRequest{
		Name:      strings.TrimSpace(c.PostForm("name")),
		Email:     strings.TrimSpace(c.PostForm("email")),
		Password:  c.PostForm("password"),
		Phone:     strings.TrimSpace(c.PostForm("phone")),
		UserType:  model.UserType(c.PostForm("userType")),
		BirthDate: c.PostForm("birthDate"),
	}

	// Validate required fields
	if req.Name == "" || req.Email == "" || req.Password == "" || req.Phone == "" || req.UserType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "semua field wajib diisi"})
		return
	}

	// Parse optional lat/lon
	if lat := c.PostForm("latitude"); lat != "" {
		if v, err := strconv.ParseFloat(lat, 64); err == nil {
			req.Latitude = &v
		}
	}
	if lon := c.PostForm("longitude"); lon != "" {
		if v, err := strconv.ParseFloat(lon, 64); err == nil {
			req.Longitude = &v
		}
	}

	// Upload KTP photo if provided
	file, header, err := c.Request.FormFile("ktp")
	if err == nil {
		defer file.Close()
		ext := strings.ToLower(filepath.Ext(header.Filename))
		objectName := fmt.Sprintf("ktp/%d%s", time.Now().UnixMilli(), ext)
		_, uploadErr := h.minio.PutObject(c.Request.Context(), h.cfg.MinioBucket, objectName,
			file, header.Size,
			minio.PutObjectOptions{ContentType: header.Header.Get("Content-Type")},
		)
		if uploadErr == nil {
			req.KTPPhoto = fmt.Sprintf("%s/%s/%s", h.cfg.MinioPublicURL, h.cfg.MinioBucket, objectName)
		}
	}

	resp, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.svc.Login(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	id := c.GetString("userID")
	resp, err := h.svc.GetMe(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) UpdateMe(c *gin.Context) {
	id := c.GetString("userID")
	var req model.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.svc.UpdateProfile(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	id := c.GetString("userID")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file tidak ditemukan"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	objectName := fmt.Sprintf("avatars/%s-%d%s", id, time.Now().UnixMilli(), ext)

	_, err = h.minio.PutObject(c.Request.Context(), h.cfg.MinioBucket, objectName,
		file, header.Size,
		minio.PutObjectOptions{ContentType: header.Header.Get("Content-Type")},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal upload foto"})
		return
	}

	avatarURL := fmt.Sprintf("%s/%s/%s", h.cfg.MinioPublicURL, h.cfg.MinioBucket, objectName)
	resp, err := h.svc.UpdateAvatar(c.Request.Context(), id, avatarURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
