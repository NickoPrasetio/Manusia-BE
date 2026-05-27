package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/booking-service/internal/model"
)

type JobRepository struct {
	db *pgxpool.Pool
}

func NewJobRepository(db *pgxpool.Pool) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS jobs (
			id             UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
			customer_id    TEXT             NOT NULL,
			customer_name  TEXT             NOT NULL DEFAULT '',
			title          TEXT             NOT NULL,
			description    TEXT             NOT NULL DEFAULT '',
			budget_per_day BIGINT           NOT NULL DEFAULT 0,
			todo_list      JSONB            NOT NULL DEFAULT '[]',
			duration_days  INT              NOT NULL DEFAULT 1,
			city           TEXT             NOT NULL DEFAULT '',
			latitude       DOUBLE PRECISION NOT NULL DEFAULT 0,
			longitude      DOUBLE PRECISION NOT NULL DEFAULT 0,
			category       TEXT             NOT NULL DEFAULT 'TASK',
			status         TEXT             NOT NULL DEFAULT 'OPEN',
			created_at     TIMESTAMPTZ      NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_jobs_status   ON jobs(status);
		CREATE INDEX IF NOT EXISTS idx_jobs_category ON jobs(category);
		CREATE INDEX IF NOT EXISTS idx_jobs_customer ON jobs(customer_id);
	`)
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanJob(row pgx.Row) (*model.Job, error) {
	var j model.Job
	var todoJSON []byte
	err := row.Scan(
		&j.ID, &j.CustomerID, &j.CustomerName,
		&j.Title, &j.Description, &j.BudgetPerDay,
		&todoJSON, &j.DurationDays,
		&j.City, &j.Latitude, &j.Longitude,
		&j.Category, &j.Status, &j.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(todoJSON, &j.TodoList)
	if j.TodoList == nil {
		j.TodoList = []string{}
	}
	return &j, nil
}

func scanJobs(rows pgx.Rows) ([]model.Job, error) {
	var result []model.Job
	for rows.Next() {
		var j model.Job
		var todoJSON []byte
		if err := rows.Scan(
			&j.ID, &j.CustomerID, &j.CustomerName,
			&j.Title, &j.Description, &j.BudgetPerDay,
			&todoJSON, &j.DurationDays,
			&j.City, &j.Latitude, &j.Longitude,
			&j.Category, &j.Status, &j.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(todoJSON, &j.TodoList)
		if j.TodoList == nil {
			j.TodoList = []string{}
		}
		result = append(result, j)
	}
	return result, rows.Err()
}

const jobCols = `id, customer_id, customer_name, title, description, budget_per_day,
	todo_list, duration_days, city, latitude, longitude, category, status, created_at`

// ── CRUD ──────────────────────────────────────────────────────────────────────

func (r *JobRepository) Create(ctx context.Context, j *model.Job) error {
	todoJSON, err := json.Marshal(j.TodoList)
	if err != nil {
		return err
	}
	return r.db.QueryRow(ctx, `
		INSERT INTO jobs
			(customer_id, customer_name, title, description, budget_per_day,
			 todo_list, duration_days, city, latitude, longitude, category)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at
	`,
		j.CustomerID, j.CustomerName, j.Title, j.Description, j.BudgetPerDay,
		todoJSON, j.DurationDays, j.City, j.Latitude, j.Longitude, j.Category,
	).Scan(&j.ID, &j.CreatedAt)
}

func (r *JobRepository) FindByID(ctx context.Context, id string) (*model.Job, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+jobCols+` FROM jobs WHERE id = $1`, id)
	j, err := scanJob(row)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("job tidak ditemukan")
	}
	return j, err
}

func (r *JobRepository) FindByCustomer(ctx context.Context, customerID string) ([]model.Job, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+jobCols+` FROM jobs WHERE customer_id = $1 ORDER BY created_at DESC`,
		customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (r *JobRepository) UpdateStatus(ctx context.Context, id string, status model.JobStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE jobs SET status=$1 WHERE id=$2`, status, id)
	return err
}

// FindNearby returns OPEN jobs within radiusKm of (lat,lon).
// Optionally filtered by category (empty string = all categories).
// Results ordered by distance ASC, limited to 50.
func (r *JobRepository) FindNearby(ctx context.Context, lat, lon, radiusKm float64, category string) ([]model.Job, error) {
	const haversine = `(6371 * acos(LEAST(1.0,
		cos(radians($1)) * cos(radians(latitude)) * cos(radians(longitude) - radians($2)) +
		sin(radians($1)) * sin(radians(latitude))
	)))`

	var query string
	var args []any

	if category != "" {
		query = fmt.Sprintf(`
			SELECT `+jobCols+`
			FROM jobs
			WHERE status    = 'OPEN'
			  AND latitude  <> 0 AND longitude <> 0
			  AND category  = $4
			  AND %s <= $3
			ORDER BY %s ASC
			LIMIT 50
		`, haversine, haversine)
		args = []any{lat, lon, radiusKm, category}
	} else {
		query = fmt.Sprintf(`
			SELECT `+jobCols+`
			FROM jobs
			WHERE status   = 'OPEN'
			  AND latitude  <> 0 AND longitude <> 0
			  AND %s <= $3
			ORDER BY %s ASC
			LIMIT 50
		`, haversine, haversine)
		args = []any{lat, lon, radiusKm}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}
