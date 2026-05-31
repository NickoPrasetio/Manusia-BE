package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manusia/review-service/internal/config"
	"github.com/manusia/review-service/internal/model"
	"github.com/manusia/review-service/internal/repository"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ReviewHandler struct {
	repo  *repository.ReviewRepository
	cfg   *config.Config
	minio *minio.Client
}

func NewReviewHandler(repo *repository.ReviewRepository, cfg *config.Config) (*ReviewHandler, error) {
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

	return &ReviewHandler{repo: repo, cfg: cfg, minio: mc}, nil
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

	reviews, total, avg, dist, err := h.repo.FindPage(c.Request.Context(), workerID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if reviews == nil {
		reviews = []model.Review{}
	}

	last := (page+1)*limit >= total

	c.JSON(http.StatusOK, model.ReviewPage{
		Reviews:   reviews,
		Total:     total,
		AvgRating: avg,
		Dist:      dist,
		Page:      page,
		Limit:     limit,
		Last:      last,
	})
}

func (h *ReviewHandler) GetByWorker(c *gin.Context) {
	workerID := c.Param("workerId")
	reviews, err := h.repo.FindByWorker(c.Request.Context(), workerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if reviews == nil {
		reviews = []model.Review{}
	}
	c.JSON(http.StatusOK, reviews)
}

func (h *ReviewHandler) CreateWithPhotos(c *gin.Context) {
	userID := c.GetString("userID")
	userName, _ := c.Get("userName")
	userNameStr, _ := userName.(string)

	workerID := c.PostForm("workerId")
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
		WorkerID:  workerID,
		UserID:    userID,
		UserName:  userNameStr,
		BookingID: bookingID,
		Rating:    rating,
		Comment:   comment,
		Photos:    photoURLs,
		Date:      time.Now().Format("2006-01-02"),
	}

	if err := h.repo.Create(c.Request.Context(), rev); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update rating di user-service secara langsung (async)
	go h.updateRating(workerID)

	c.JSON(http.StatusCreated, rev)
}

// updateRating menghitung ulang rata-rata rating dan PATCH ke user-service
func (h *ReviewHandler) updateRating(workerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	avg, count, err := h.repo.AverageRating(ctx, workerID)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/api/internal/users/%s/rating", h.cfg.UserServiceURL, workerID)
	body, _ := json.Marshal(map[string]interface{}{
		"rating":       avg,
		"totalReviews": count,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	_, _ = client.Do(req)
}
