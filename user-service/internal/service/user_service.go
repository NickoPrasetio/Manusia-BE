package service

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/manusia/user-service/internal/config"
	"github.com/manusia/user-service/internal/model"
	"github.com/manusia/user-service/internal/repository"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type UserService struct {
	repo  *repository.ProfileRepository
	cfg   *config.Config
	minio *minio.Client
}

func NewUserService(repo *repository.ProfileRepository, cfg *config.Config) (*UserService, error) {
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
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, cfg.MinioBucket)
		_ = mc.SetBucketPolicy(ctx, cfg.MinioBucket, policy)
	}

	return &UserService{repo: repo, cfg: cfg, minio: mc}, nil
}

func (s *UserService) ListAll(ctx context.Context, search string, available *bool) ([]model.UserProfile, error) {
	return s.repo.FindAll(ctx, search, available)
}

func (s *UserService) ListPage(ctx context.Context, page, size int, search string, available *bool) (*model.ProfilePage, error) {
	return s.repo.FindPage(ctx, page, size, search, available)
}

func (s *UserService) GetByID(ctx context.Context, id string) (*model.UserProfile, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *UserService) GetByAuthID(ctx context.Context, authID string) (*model.UserProfile, error) {
	return s.repo.FindByAuthID(ctx, authID)
}

func (s *UserService) Create(ctx context.Context, req *model.CreateProfileRequest) (*model.UserProfile, error) {
	specs := req.Specializations
	if specs == nil {
		specs = []string{}
	}
	p := &model.UserProfile{
		AuthID:          req.AuthID,
		Name:            req.Name,
		Age:             req.Age,
		Experience:      req.Experience,
		Specializations: specs,
		Location:        req.Location,
		PricePerDay:     req.PricePerDay,
		Bio:             req.Bio,
		WorkStatus:      model.WorkStatusOpen,
		IsAvailable:     true,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *UserService) Update(ctx context.Context, id string, req *model.UpdateProfileRequest) (*model.UserProfile, error) {
	if err := s.repo.Update(ctx, id, req); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

func (s *UserService) UploadPhotoBytes(ctx context.Context, id string, data []byte, filename, contentType string) (*model.UserProfile, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	objectName := fmt.Sprintf("profiles/%s-%d%s", id, time.Now().UnixMilli(), ext)

	reader := bytes.NewReader(data)
	_, err := s.minio.PutObject(ctx, s.cfg.MinioBucket, objectName, reader, int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return nil, fmt.Errorf("gagal upload foto")
	}

	avatarURL := fmt.Sprintf("%s/%s/%s", s.cfg.MinioPublicURL, s.cfg.MinioBucket, objectName)
	if err := s.repo.UpdateAvatar(ctx, id, avatarURL); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

func (s *UserService) UpdateAvailability(ctx context.Context, authID string, isAvailable bool, lat, lon *float64) (*model.UserProfile, error) {
	return s.repo.UpdateAvailabilityByAuthID(ctx, authID, isAvailable, lat, lon)
}

func (s *UserService) UpdateRating(ctx context.Context, authID string, rating float64, totalReviews int) error {
	return s.repo.UpdateRating(ctx, authID, rating, totalReviews)
}
