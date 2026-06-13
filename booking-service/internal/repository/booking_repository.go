package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/booking-service/internal/model"
)

type BookingRepository struct {
	db *pgxpool.Pool
}

func NewBookingRepository(db *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{db: db}
}

func (r *BookingRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS bookings (
			id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			worker_id      TEXT NOT NULL,
			worker_name    TEXT NOT NULL DEFAULT '',
			worker_avatar  TEXT NOT NULL DEFAULT '',
			customer_id    TEXT NOT NULL,
			customer_name  TEXT NOT NULL,
			address        TEXT NOT NULL,
			city           TEXT NOT NULL,
			latitude       DOUBLE PRECISION NOT NULL DEFAULT 0,
			longitude      DOUBLE PRECISION NOT NULL DEFAULT 0,
			booking_date   TEXT NOT NULL,
			start_time     TEXT NOT NULL,
			duration_days  INT NOT NULL DEFAULT 1,
			payment_method TEXT NOT NULL DEFAULT 'CASH',
			status         TEXT NOT NULL DEFAULT 'PENDING',
			notes          TEXT NOT NULL DEFAULT '',
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}
	// Idempotent migrations
	r.db.Exec(ctx, `ALTER TABLE bookings ADD COLUMN IF NOT EXISTS job_id    TEXT NOT NULL DEFAULT ''`)
	r.db.Exec(ctx, `ALTER TABLE bookings ADD COLUMN IF NOT EXISTS job_title TEXT NOT NULL DEFAULT ''`)
	return nil
}

func (r *BookingRepository) Create(ctx context.Context, b *model.Booking) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO bookings (worker_id, worker_name, worker_avatar, customer_id, customer_name, address, city, latitude, longitude, booking_date, start_time, duration_days, payment_method, notes, job_id, job_title)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id, created_at
	`, b.WorkerID, b.WorkerName, b.WorkerAvatar, b.CustomerID, b.CustomerName,
		b.Address, b.City, b.Latitude, b.Longitude,
		b.BookingDate, b.StartTime, b.DurationDays, b.PaymentMethod, b.Notes, b.JobID, b.JobTitle).
		Scan(&b.ID, &b.CreatedAt)
}

func (r *BookingRepository) FindByCustomer(ctx context.Context, customerID string) ([]model.Booking, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, worker_id, worker_name, worker_avatar, customer_id, customer_name, address, city, latitude, longitude, booking_date, start_time, duration_days, payment_method, status, notes, job_id, job_title, created_at
		FROM bookings WHERE customer_id = $1 ORDER BY created_at DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookings(rows)
}

func (r *BookingRepository) FindByWorker(ctx context.Context, workerID string) ([]model.Booking, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, worker_id, worker_name, worker_avatar, customer_id, customer_name, address, city, latitude, longitude, booking_date, start_time, duration_days, payment_method, status, notes, job_id, job_title, created_at
		FROM bookings WHERE worker_id = $1 ORDER BY created_at DESC
	`, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookings(rows)
}

func (r *BookingRepository) FindByID(ctx context.Context, id string) (*model.Booking, error) {
	var b model.Booking
	err := r.db.QueryRow(ctx, `
		SELECT id, worker_id, worker_name, worker_avatar, customer_id, customer_name, address, city, latitude, longitude, booking_date, start_time, duration_days, payment_method, status, notes, job_id, job_title, created_at
		FROM bookings WHERE id = $1
	`, id).Scan(
		&b.ID, &b.WorkerID, &b.WorkerName, &b.WorkerAvatar,
		&b.CustomerID, &b.CustomerName, &b.Address, &b.City,
		&b.Latitude, &b.Longitude, &b.BookingDate, &b.StartTime,
		&b.DurationDays, &b.PaymentMethod, &b.Status, &b.Notes, &b.JobID, &b.JobTitle, &b.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("booking tidak ditemukan")
	}
	return &b, err
}

func (r *BookingRepository) UpdateStatus(ctx context.Context, id string, status model.BookingStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE bookings SET status=$1 WHERE id=$2`, status, id)
	return err
}

// FindOpenNearby returns PENDING bookings within radiusKm of the given coordinates,
// ordered by distance ascending. Uses the Haversine formula.
func (r *BookingRepository) FindOpenNearby(ctx context.Context, lat, lon, radiusKm float64) ([]model.Booking, error) {
	const haversine = `(6371 * acos(LEAST(1.0,
		cos(radians($1)) * cos(radians(latitude)) * cos(radians(longitude) - radians($2)) +
		sin(radians($1)) * sin(radians(latitude))
	)))`
	query := fmt.Sprintf(`
		SELECT id, worker_id, worker_name, worker_avatar, customer_id, customer_name,
		       address, city, latitude, longitude, booking_date, start_time, duration_days,
		       payment_method, status, notes, job_id, job_title, created_at
		FROM bookings
		WHERE status = 'PENDING'
		  AND latitude  <> 0 AND longitude <> 0
		  AND %s <= $3
		ORDER BY %s ASC
		LIMIT 50
	`, haversine, haversine)

	rows, err := r.db.Query(ctx, query, lat, lon, radiusKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookings(rows)
}

func scanBookings(rows pgx.Rows) ([]model.Booking, error) {
	var result []model.Booking
	for rows.Next() {
		var b model.Booking
		if err := rows.Scan(
			&b.ID, &b.WorkerID, &b.WorkerName, &b.WorkerAvatar,
			&b.CustomerID, &b.CustomerName, &b.Address, &b.City,
			&b.Latitude, &b.Longitude, &b.BookingDate, &b.StartTime,
			&b.DurationDays, &b.PaymentMethod, &b.Status, &b.Notes, &b.JobID, &b.JobTitle, &b.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}
