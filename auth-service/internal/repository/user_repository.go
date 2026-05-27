package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/auth-service/internal/model"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name          TEXT NOT NULL,
			email         TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			phone         TEXT NOT NULL DEFAULT '',
			role          TEXT NOT NULL DEFAULT 'ROLE_USER',
			user_type     TEXT NOT NULL DEFAULT 'CUSTOMER',
			avatar        TEXT NOT NULL DEFAULT '',
			birth_date    TEXT NOT NULL DEFAULT '',
			ktp_photo     TEXT NOT NULL DEFAULT '',
			latitude      DOUBLE PRECISION,
			longitude     DOUBLE PRECISION,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}
	// Add columns for existing deployments (idempotent)
	r.db.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS birth_date TEXT NOT NULL DEFAULT ''`)
	r.db.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS ktp_photo  TEXT NOT NULL DEFAULT ''`)
	return nil
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash, phone, role, user_type, birth_date, ktp_photo, latitude, longitude)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`, u.Name, u.Email, u.PasswordHash, u.Phone, u.Role, u.UserType,
		u.BirthDate, u.KTPPhoto, u.Latitude, u.Longitude).
		Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("email sudah terdaftar")
		}
		return err
	}
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, name, email, password_hash, phone, role, user_type, avatar, birth_date, ktp_photo, latitude, longitude, created_at
		FROM users WHERE email = $1
	`, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash,
		&u.Phone, &u.Role, &u.UserType, &u.Avatar,
		&u.BirthDate, &u.KTPPhoto,
		&u.Latitude, &u.Longitude, &u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("email atau password salah")
	}
	return u, err
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, name, email, password_hash, phone, role, user_type, avatar, birth_date, ktp_photo, latitude, longitude, created_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash,
		&u.Phone, &u.Role, &u.UserType, &u.Avatar,
		&u.BirthDate, &u.KTPPhoto,
		&u.Latitude, &u.Longitude, &u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("user tidak ditemukan")
	}
	return u, err
}

func (r *UserRepository) UpdateProfile(ctx context.Context, id, name, phone string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET name = $1, phone = $2 WHERE id = $3`,
		name, phone, id,
	)
	return err
}

func (r *UserRepository) UpdateAvatar(ctx context.Context, id, avatarURL string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET avatar = $1 WHERE id = $2`,
		avatarURL, id,
	)
	return err
}

func isUniqueViolation(err error) bool {
	return err != nil && (
		contains(err.Error(), "duplicate key") ||
		contains(err.Error(), "unique constraint"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
