package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manusia/review-service/internal/config"
	"github.com/manusia/review-service/internal/model"
	"github.com/manusia/review-service/internal/service"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ReviewHandler struct {
	svc   *service.ReviewService
	cfg   *config.Config
	minio *minio.Client
}

func NewReviewHandler(svc *service.ReviewService, cfg *config.Config) (*ReviewHandler, error) {
	mc, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccess, cfg.MinioSecret, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	exists, _ := mc.BucketExists(ctx, cfg.MinioBucket)
	if !exists {
		_ = mc.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{})
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, cfg.MinioBucket)
		_ = mc.SetBucketPolicy(ctx, cfg.MinioBucket, policy)
	}

	return &ReviewHandler{svc: svc, cfg: cfg, minio: mc}, nil
}

// GetByWorkerPage handles GET /api/reviews/worker/:workerId/page?page=0&limit=10
func (h *ReviewHandler) GetByWorkerPage(c *gin.Context) {
	workerID := c.Param("workerId")

	page := 0
	limit := 10
	fmt.Sscanf(c.DefaultQuery("page", "0"), "%d", &page)
	fmt.Sscanf(c.DefaultQuery("limit", "10"), "%d", &limit)
	if limit < 1 || limit > 50 {
		limit = 10
	}
	if page < 0 {
		page = 0
	}

	result, err := h.svc.GetByWorkerPage(c.Request.Context(), workerID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReviewHandler) GetByWorker(c *gin.Context) {
	workerID := c.Param("workerId")
	reviews, err := h.svc.GetByWorker(c.Request.Context(), workerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reviews)
}

// GetGivenByUser handles GET /api/reviews/given/:userId/page?page=0&limit=10
func (h *ReviewHandler) GetGivenByUser(c *gin.Context) {
	userID := c.Param("userId")

	page, limit := 0, 10
	fmt.Sscanf(c.DefaultQuery("page", "0"), "%d", &page)
	fmt.Sscanf(c.DefaultQuery("limit", "10"), "%d", &limit)
	if limit < 1 || limit > 50 {
		limit = 10
	}
	if page < 0 {
		page = 0
	}

	result, err := h.svc.GetGivenByUser(c.Request.Context(), userID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReviewHandler) CreateWithPhotos(c *gin.Context) {
	userID := c.GetString("userID")
	userName, _ := c.Get("userName")
	userNameStr, _ := userName.(string)

	workerID := c.PostForm("workerId")
	workerName := c.PostForm("workerName")
	workerAvatar := c.PostForm("workerAvatar")
	bookingID := c.PostForm("bookingId")
	ratingStr := c.PostForm("rating")
	comment := c.PostForm("comment")

	if workerID == "" || ratingStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workerId dan rating wajib diisi"})
		return
	}

	var rating int
	fmt.Sscanf(ratingStr, "%d", &rating)
	if rating < 1 || rating > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rating harus 1-5"})
		return
	}

	// Upload photos ke MinIO
	form, _ := c.MultipartForm()
	var photoURLs []string
	if form != nil {
		files := form.File["photos"]
		for _, fh := range files {
			f, err := fh.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(f)
			f.Close()

			ext := strings.ToLower(filepath.Ext(fh.Filename))
			objName := fmt.Sprintf("reviews/%s-%d%s", userID, time.Now().UnixNano(), ext)
			reader := bytes.NewReader(data)
			_, err = h.minio.PutObject(c.Request.Context(), h.cfg.MinioBucket, objName, reader, int64(len(data)),
				minio.PutObjectOptions{ContentType: fh.Header.Get("Content-Type")})
			if err == nil {
				url := fmt.Sprintf("%s/%s/%s", h.cfg.MinioPublicURL, h.cfg.MinioBucket, objName)
				photoURLs = append(photoURLs, url)
			}
		}
	}

	rev := &model.Review{
		WorkerID:     workerID,
		WorkerName:   workerName,
		WorkerAvatar: workerAvatar,
		UserID:       userID,
		UserName:     userNameStr,
		BookingID:    bookingID,
		Rating:       rating,
		Comment:      comment,
		Photos:       photoURLs,
		Date:         time.Now().Format("2006-01-02"),
	}

	if err := h.svc.CreateReview(c.Request.Context(), rev); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rev)
}

// EditReview handles PATCH /api/reviews/:id (multipart/form-data)
func (h *ReviewHandler) EditReview(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userID")

	ratingStr := c.PostForm("rating")
	comment := c.PostForm("comment")

	var rating int
	fmt.Sscanf(ratingStr, "%d", &rating)
	if rating < 1 || rating > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rating (1-5) wajib diisi"})
		return
	}

	// Upload new photos if provided
	var photos *model.StringArray
	form, _ := c.MultipartForm()
	if form != nil {
		if files := form.File["photos"]; len(files) > 0 {
			urls := make(model.StringArray, 0, len(files))
			for _, fh := range files {
				f, err := fh.Open()
				if err != nil {
					continue
				}
				data, _ := io.ReadAll(f)
				f.Close()
				ext := strings.ToLower(filepath.Ext(fh.Filename))
				objName := fmt.Sprintf("reviews/%s-%d%s", userID, time.Now().UnixNano(), ext)
				reader := bytes.NewReader(data)
				_, err = h.minio.PutObject(c.Request.Context(), h.cfg.MinioBucket, objName, reader, int64(len(data)),
					minio.PutObjectOptions{ContentType: fh.Header.Get("Content-Type")})
				if err == nil {
					urls = append(urls, fmt.Sprintf("%s/%s/%s", h.cfg.MinioPublicURL, h.cfg.MinioBucket, objName))
				}
			}
			photos = &urls
		}
	}

	rev, err := h.svc.Edit(c.Request.Context(), id, userID, rating, comment, photos)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrEditLimitReach), errors.Is(err, service.ErrEditExpired):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, rev)
}
