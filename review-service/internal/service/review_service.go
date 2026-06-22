package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/manusia/review-service/internal/model"
)

const (
	maxEditCount  = 2
	editWindowDay = 3 * 24 * time.Hour
)

var (
	ErrNotFound       = errors.New("ulasan tidak ditemukan")
	ErrForbidden      = errors.New("kamu bukan pembuat ulasan ini")
	ErrEditLimitReach = errors.New("ulasan sudah diedit 2 kali, tidak bisa diedit lagi")
	ErrEditExpired    = errors.New("ulasan hanya bisa diedit dalam 3 hari setelah dibuat")
)

// ReviewRepository is the persistence contract ReviewService depends on —
// satisfied implicitly by *repository.ReviewRepository.
type ReviewRepository interface {
	FindByID(ctx context.Context, id string) (*model.Review, error)
	Update(ctx context.Context, id string, rating int, comment string, photos *model.StringArray) error
	FindByWorker(ctx context.Context, workerID string) ([]model.Review, error)
	FindPage(ctx context.Context, workerID string, page, limit int) ([]model.Review, int, float64, []model.RatingDist, error)
	FindGivenByUser(ctx context.Context, userID string, page, limit int) ([]model.Review, int, error)
	Create(ctx context.Context, rev *model.Review) error
	AverageRating(ctx context.Context, workerID string) (float64, int, error)
}

type ReviewService struct {
	repo           ReviewRepository
	userServiceURL string
}

func NewReviewService(repo ReviewRepository, userServiceURL string) *ReviewService {
	return &ReviewService{repo: repo, userServiceURL: userServiceURL}
}

func (s *ReviewService) GetByWorker(ctx context.Context, workerID string) ([]model.Review, error) {
	reviews, err := s.repo.FindByWorker(ctx, workerID)
	if err != nil {
		return nil, err
	}
	if reviews == nil {
		reviews = []model.Review{}
	}
	return reviews, nil
}

func (s *ReviewService) GetByWorkerPage(ctx context.Context, workerID string, page, limit int) (*model.ReviewPage, error) {
	reviews, total, avg, dist, err := s.repo.FindPage(ctx, workerID, page, limit)
	if err != nil {
		return nil, err
	}
	if reviews == nil {
		reviews = []model.Review{}
	}
	return &model.ReviewPage{
		Reviews:   reviews,
		Total:     total,
		AvgRating: avg,
		Dist:      dist,
		Page:      page,
		Limit:     limit,
		Last:      (page+1)*limit >= total,
	}, nil
}

func (s *ReviewService) GetGivenByUser(ctx context.Context, userID string, page, limit int) (*model.GivenReviewPage, error) {
	reviews, total, err := s.repo.FindGivenByUser(ctx, userID, page, limit)
	if err != nil {
		return nil, err
	}
	if reviews == nil {
		reviews = []model.Review{}
	}
	return &model.GivenReviewPage{
		Reviews: reviews,
		Total:   total,
		Page:    page,
		Limit:   limit,
		Last:    (page+1)*limit >= total,
	}, nil
}

// CreateReview persists a new review and asynchronously refreshes the
// worker's average rating in user-service.
func (s *ReviewService) CreateReview(ctx context.Context, rev *model.Review) error {
	if err := s.repo.Create(ctx, rev); err != nil {
		return err
	}
	go s.updateRating(rev.WorkerID)
	return nil
}

func (s *ReviewService) Edit(ctx context.Context, id, userID string, rating int, comment string, photos *model.StringArray) (*model.Review, error) {
	rev, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if rev.UserID != userID {
		return nil, ErrForbidden
	}
	if rev.EditCount >= maxEditCount {
		return nil, ErrEditLimitReach
	}
	if time.Since(rev.CreatedAt) > editWindowDay {
		return nil, ErrEditExpired
	}

	if err := s.repo.Update(ctx, id, rating, comment, photos); err != nil {
		return nil, err
	}

	rev.Rating = rating
	rev.Comment = comment
	rev.EditCount++
	if photos != nil {
		rev.Photos = *photos
	}

	go s.updateRating(rev.WorkerID)
	return rev, nil
}

// updateRating recomputes the worker's average rating and PATCHes it to user-service.
func (s *ReviewService) updateRating(workerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	avg, count, err := s.repo.AverageRating(ctx, workerID)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/api/internal/users/%s/rating", s.userServiceURL, workerID)
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
