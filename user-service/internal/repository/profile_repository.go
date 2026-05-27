package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/user-service/internal/model"
)

type ProfileRepository struct {
	db *pgxpool.Pool
}

func NewProfileRepository(db *pgxpool.Pool) *ProfileRepository {
	return &ProfileRepository{db: db}
}

func (r *ProfileRepository) Migrate(ctx context.Context) error {
	if _, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS user_profiles (
			id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			auth_id          TEXT UNIQUE NOT NULL,
			name             TEXT NOT NULL DEFAULT '',
			avatar           TEXT NOT NULL DEFAULT '',
			age              INT  NOT NULL DEFAULT 0,
			experience       INT  NOT NULL DEFAULT 0,
			rating           DOUBLE PRECISION NOT NULL DEFAULT 0,
			total_reviews    INT  NOT NULL DEFAULT 0,
			specializations  JSONB NOT NULL DEFAULT '[]',
			location         TEXT NOT NULL DEFAULT '',
			price_per_day    BIGINT NOT NULL DEFAULT 0,
			is_available     BOOLEAN NOT NULL DEFAULT TRUE,
			work_status      TEXT NOT NULL DEFAULT 'OPEN',
			bio              TEXT NOT NULL DEFAULT '',
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return err
	}
	// Idempotent: add lat/lon columns for existing deployments
	for _, col := range []string{
		`ALTER TABLE user_profiles ADD COLUMN IF NOT EXISTS latitude  DOUBLE PRECISION`,
		`ALTER TABLE user_profiles ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION`,
	} {
		if _, err := r.db.Exec(ctx, col); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProfileRepository) Create(ctx context.Context, p *model.UserProfile) error {
	specs, _ := json.Marshal(p.Specializations)
	return r.db.QueryRow(ctx, `
		INSERT INTO user_profiles (auth_id, name, avatar, age, experience, specializations, location, price_per_day, bio)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`, p.AuthID, p.Name, p.Avatar, p.Age, p.Experience, string(specs), p.Location, p.PricePerDay, p.Bio).
		Scan(&p.ID, &p.CreatedAt)
}

func (r *ProfileRepository) FindAll(ctx context.Context, search string, available *bool) ([]model.UserProfile, error) {
	q := baseSelect() + ` WHERE 1=1`
	args, _ := buildFilters(&q, search, available, 1)
	q += " ORDER BY rating DESC"

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (r *ProfileRepository) FindPage(ctx context.Context, page, size int, search string, available *bool) (*model.ProfilePage, error) {
	countQ := `SELECT COUNT(*) FROM user_profiles WHERE 1=1`
	dataQ := baseSelect() + ` WHERE 1=1`
	args, idx := buildFilters(&countQ, search, available, 1)
	buildFiltersInto(&dataQ, search, available, 1)

	var total int
	if err := r.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, err
	}

	dataArgs, nextIdx := buildFiltersSlice(search, available, 1)
	dataQ += fmt.Sprintf(" ORDER BY rating DESC LIMIT $%d OFFSET $%d", nextIdx, nextIdx+1)
	dataArgs = append(dataArgs, size, page*size)

	_ = idx // suppress unused warning

	rows, err := r.db.Query(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles, err := scanRows(rows)
	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(size)))
	return &model.ProfilePage{
		Content:       profiles,
		TotalElements: total,
		TotalPages:    totalPages,
		Number:        page,
		Last:          page >= totalPages-1,
	}, nil
}

func (r *ProfileRepository) FindByID(ctx context.Context, id string) (*model.UserProfile, error) {
	return r.scanOne(ctx, baseSelect()+` WHERE id = $1`, id)
}

func (r *ProfileRepository) FindByAuthID(ctx context.Context, authID string) (*model.UserProfile, error) {
	return r.scanOne(ctx, baseSelect()+` WHERE auth_id = $1`, authID)
}

func (r *ProfileRepository) Update(ctx context.Context, id string, req *model.UpdateProfileRequest) error {
	setClauses := []string{}
	args := []interface{}{}
	idx := 1

	if req.Name != "" {
		setClauses = append(setClauses, fmt.Sprintf("name=$%d", idx)); args = append(args, req.Name); idx++
	}
	if req.Age > 0 {
		setClauses = append(setClauses, fmt.Sprintf("age=$%d", idx)); args = append(args, req.Age); idx++
	}
	if req.Experience > 0 {
		setClauses = append(setClauses, fmt.Sprintf("experience=$%d", idx)); args = append(args, req.Experience); idx++
	}
	if len(req.Specializations) > 0 {
		specs, _ := json.Marshal(req.Specializations)
		setClauses = append(setClauses, fmt.Sprintf("specializations=$%d", idx)); args = append(args, string(specs)); idx++
	}
	if req.Location != "" {
		setClauses = append(setClauses, fmt.Sprintf("location=$%d", idx)); args = append(args, req.Location); idx++
	}
	if req.PricePerDay > 0 {
		setClauses = append(setClauses, fmt.Sprintf("price_per_day=$%d", idx)); args = append(args, req.PricePerDay); idx++
	}
	if req.Bio != "" {
		setClauses = append(setClauses, fmt.Sprintf("bio=$%d", idx)); args = append(args, req.Bio); idx++
	}
	if req.IsAvailable != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_available=$%d", idx)); args = append(args, *req.IsAvailable); idx++
	}
	if req.WorkStatus != "" {
		setClauses = append(setClauses, fmt.Sprintf("work_status=$%d", idx)); args = append(args, req.WorkStatus); idx++
	}
	if len(setClauses) == 0 {
		return nil
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE user_profiles SET %s WHERE id=$%d", strings.Join(setClauses, ","), idx)
	_, err := r.db.Exec(ctx, q, args...)
	return err
}

// UpdateAvailabilityByAuthID sets is_available + work_status (and optionally lat/lon) for the
// given auth user. Uses UPSERT so the first toggle creates a minimal profile row when one
// doesn't yet exist. Lat/lon are only written when non-nil; existing values are preserved.
func (r *ProfileRepository) UpdateAvailabilityByAuthID(ctx context.Context, authID string, isAvailable bool, lat, lon *float64) (*model.UserProfile, error) {
	workStatus := "CLOSED"
	if isAvailable {
		workStatus = "OPEN"
	}
	var p model.UserProfile
	var specsJSON []byte
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_profiles (auth_id, is_available, work_status, latitude, longitude)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (auth_id) DO UPDATE SET
			is_available = EXCLUDED.is_available,
			work_status  = EXCLUDED.work_status,
			latitude     = COALESCE(EXCLUDED.latitude,  user_profiles.latitude),
			longitude    = COALESCE(EXCLUDED.longitude, user_profiles.longitude)
		RETURNING id, auth_id, name, avatar, age, experience, rating, total_reviews,
		          specializations, location, price_per_day, is_available, work_status,
		          bio, latitude, longitude, created_at
	`, authID, isAvailable, workStatus, lat, lon).Scan(
		&p.ID, &p.AuthID, &p.Name, &p.Avatar, &p.Age, &p.Experience,
		&p.Rating, &p.TotalReviews, &specsJSON, &p.Location,
		&p.PricePerDay, &p.IsAvailable, &p.WorkStatus, &p.Bio, &p.Latitude, &p.Longitude, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(specsJSON, &p.Specializations)
	if p.Specializations == nil {
		p.Specializations = []string{}
	}
	return &p, nil
}

func (r *ProfileRepository) UpdateAvatar(ctx context.Context, id, avatarURL string) error {
	_, err := r.db.Exec(ctx, `UPDATE user_profiles SET avatar=$1 WHERE id=$2`, avatarURL, id)
	return err
}

func (r *ProfileRepository) UpdateRating(ctx context.Context, authID string, rating float64, totalReviews int) error {
	_, err := r.db.Exec(ctx, `UPDATE user_profiles SET rating=$1, total_reviews=$2 WHERE auth_id=$3`, rating, totalReviews, authID)
	return err
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func baseSelect() string {
	return `SELECT id, auth_id, name, avatar, age, experience, rating, total_reviews, specializations, location, price_per_day, is_available, work_status, bio, latitude, longitude, created_at FROM user_profiles`
}

func buildFilters(q *string, search string, available *bool, startIdx int) ([]interface{}, int) {
	args := []interface{}{}
	idx := startIdx
	if search != "" {
		*q += fmt.Sprintf(" AND (name ILIKE $%d OR location ILIKE $%d OR bio ILIKE $%d)", idx, idx, idx)
		args = append(args, "%"+search+"%")
		idx++
	}
	if available != nil && *available {
		*q += fmt.Sprintf(" AND is_available = $%d", idx)
		args = append(args, true)
		idx++
	}
	return args, idx
}

func buildFiltersInto(q *string, search string, available *bool, startIdx int) {
	idx := startIdx
	if search != "" {
		*q += fmt.Sprintf(" AND (name ILIKE $%d OR location ILIKE $%d OR bio ILIKE $%d)", idx, idx, idx)
		idx++
	}
	if available != nil && *available {
		*q += fmt.Sprintf(" AND is_available = $%d", idx)
		idx++
	}
	_ = idx
}

func buildFiltersSlice(search string, available *bool, startIdx int) ([]interface{}, int) {
	args := []interface{}{}
	idx := startIdx
	if search != "" {
		args = append(args, "%"+search+"%")
		idx++
	}
	if available != nil && *available {
		args = append(args, true)
		idx++
	}
	return args, idx
}

func (r *ProfileRepository) scanOne(ctx context.Context, q string, arg interface{}) (*model.UserProfile, error) {
	var p model.UserProfile
	var specsJSON []byte
	err := r.db.QueryRow(ctx, q, arg).Scan(
		&p.ID, &p.AuthID, &p.Name, &p.Avatar, &p.Age, &p.Experience,
		&p.Rating, &p.TotalReviews, &specsJSON, &p.Location,
		&p.PricePerDay, &p.IsAvailable, &p.WorkStatus, &p.Bio, &p.Latitude, &p.Longitude, &p.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user tidak ditemukan")
		}
		return nil, err
	}
	_ = json.Unmarshal(specsJSON, &p.Specializations)
	if p.Specializations == nil {
		p.Specializations = []string{}
	}
	return &p, nil
}

func scanRows(rows pgx.Rows) ([]model.UserProfile, error) {
	var result []model.UserProfile
	for rows.Next() {
		var p model.UserProfile
		var specsJSON []byte
		err := rows.Scan(
			&p.ID, &p.AuthID, &p.Name, &p.Avatar, &p.Age, &p.Experience,
			&p.Rating, &p.TotalReviews, &specsJSON, &p.Location,
			&p.PricePerDay, &p.IsAvailable, &p.WorkStatus, &p.Bio, &p.Latitude, &p.Longitude, &p.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal(specsJSON, &p.Specializations)
		if p.Specializations == nil {
			p.Specializations = []string{}
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
