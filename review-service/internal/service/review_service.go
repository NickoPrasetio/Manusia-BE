package service

import (
	"context"
	"errors"
	"time"

	"github.com/manusia/review-service/internal/model"
	"github.com/manusia/review-service/internal/repository"
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

type IReviewService interface {
	Edit(ctx context.Context, id, userID string, rating int, comment string, photos *model.StringArray) (*model.Review, error)
}

type ReviewService struct {
	repo *repository.ReviewRepository
}

func NewReviewService(repo *repository.ReviewRepository) *ReviewService {
	return &ReviewService{repo: repo}
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
	return rev, nil
}
