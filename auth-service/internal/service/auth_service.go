package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/manusia/auth-service/internal/model"
	"github.com/manusia/auth-service/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo      *repository.UserRepository
	jwtSecret []byte
}

func NewAuthService(repo *repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{repo: repo, jwtSecret: []byte(jwtSecret)}
}

func (s *AuthService) Register(ctx context.Context, req *model.RegisterRequest) (*model.AuthResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &model.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		Phone:        req.Phone,
		Role:         "ROLE_USER",
		NIK:          req.NIK,
		BirthDate:    req.BirthDate,
		Gender:       req.Gender,
		KTPPhoto:     req.KTPPhoto,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	token, err := s.generateToken(u)
	if err != nil {
		return nil, err
	}

	return toResponse(u, token), nil
}

func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest) (*model.AuthResponse, error) {
	u, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("email atau password salah")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("email atau password salah")
	}

	token, err := s.generateToken(u)
	if err != nil {
		return nil, err
	}

	return toResponse(u, token), nil
}

func (s *AuthService) GetMe(ctx context.Context, id string) (*model.AuthResponse, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toResponse(u, ""), nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, id string, req *model.UpdateProfileRequest) (*model.AuthResponse, error) {
	if err := s.repo.UpdateProfile(ctx, id, req.Name, req.Phone); err != nil {
		return nil, err
	}
	return s.GetMe(ctx, id)
}

func (s *AuthService) ChangePassword(ctx context.Context, id, currentPassword, newPassword string) error {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("user tidak ditemukan")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("password saat ini salah")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, id, string(newHash))
}

func (s *AuthService) UpdateAvatar(ctx context.Context, id, avatarURL string) (*model.AuthResponse, error) {
	if err := s.repo.UpdateAvatar(ctx, id, avatarURL); err != nil {
		return nil, err
	}
	return s.GetMe(ctx, id)
}

func (s *AuthService) generateToken(u *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   u.ID,
		"email": u.Email,
		"name":  u.Name,
		"role":  u.Role,
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) JWTSecret() []byte {
	return s.jwtSecret
}

func toResponse(u *model.User, token string) *model.AuthResponse {
	return &model.AuthResponse{
		Token:     token,
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Phone:     u.Phone,
		Role:      u.Role,
		Avatar:    u.Avatar,
		NIK:       u.NIK,
		BirthDate: u.BirthDate,
		Gender:    u.Gender,
		KTPPhoto:  u.KTPPhoto,
		Latitude:  u.Latitude,
		Longitude: u.Longitude,
	}
}
