package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/review-service/internal/model"
)

type ReviewRepository struct {
	db *pgxpool.Pool
}

func NewReviewRepository(db *pgxpool.Pool) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS reviews (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			worker_id  TEXT NOT NULL,
			user_id    TEXT NOT NULL,
			user_name  TEXT NOT NULL,
			booking_id TEXT NOT NULL DEFAULT '',
			rating     INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
			comment    TEXT NOT NULL DEFAULT '',
			photos     JSONB NOT NULL DEFAULT '[]',
			date       TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func (r *ReviewRepository) Create(ctx context.Context, rev *model.Review) error {
	photos, _ := json.Marshal(rev.Photos)
	return r.db.QueryRow(ctx, `
		INSERT INTO reviews (worker_id, user_id, user_name, booking_id, rating, comment, photos, date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at
	`, rev.WorkerID, rev.UserID, rev.UserName, rev.BookingID, rev.Rating, rev.Comment, string(photos), rev.Date).
		Scan(&rev.ID, &rev.CreatedAt)
}

func (r *ReviewRepository) FindByWorker(ctx context.Context, workerID string) ([]model.Review, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, worker_id, user_id, user_name, booking_id, rating, comment, photos, date, created_at
		FROM reviews WHERE worker_id=$1 ORDER BY created_at DESC
	`, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReviews(rows)
}

// FindPage returns a paginated slice of reviews for a worker, newest-first,
// plus aggregate stats (total, avg, per-star distribution).
func (r *ReviewRepository) FindPage(ctx context.Context, workerID string, page, limit int) ([]model.Review, int, float64, []model.RatingDist, error) {
	// aggregate stats
	rows, err := r.db.Query(ctx,
		`SELECT rating, COUNT(*) FROM reviews WHERE worker_id=$1 GROUP BY rating`, workerID)
	if err != nil {
		return nil, 0, 0, nil, err
	}
	defer rows.Close()

	var total int
	var sum int
	distMap := map[int]int{}
	for rows.Next() {
		var star, cnt int
		if err := rows.Scan(&star, &cnt); err != nil {
			return nil, 0, 0, nil, err
		}
		distMap[star] = cnt
		total += cnt
		sum += star * cnt
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, nil, err
	}

	var avg float64
	if total > 0 {
		avg = float64(sum) / float64(total)
	}

	dist := make([]model.RatingDist, 5)
	for i, star := range []int{5, 4, 3, 2, 1} {
		dist[i] = model.RatingDist{Star: star, Count: distMap[star]}
	}

	// paginated reviews
	offset := page * limit
	reviewRows, err := r.db.Query(ctx,
		`SELECT id, worker_id, user_id, user_name, booking_id, rating, comment, photos, date, created_at
		 FROM reviews WHERE worker_id=$1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`,
		workerID, limit, offset)
	if err != nil {
		return nil, 0, 0, nil, err
	}
	defer reviewRows.Close()

	reviews, err := scanReviews(reviewRows)
	if err != nil {
		return nil, 0, 0, nil, err
	}
	return reviews, total, avg, dist, nil
}

func (r *ReviewRepository) AverageRating(ctx context.Context, workerID string) (float64, int, error) {
	var avg float64
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(AVG(rating),0), COUNT(*) FROM reviews WHERE worker_id=$1`, workerID,
	).Scan(&avg, &count)
	return avg, count, err
}

func scanReviews(rows pgx.Rows) ([]model.Review, error) {
	var result []model.Review
	for rows.Next() {
		var rev model.Review
		var photosJSON []byte
		err := rows.Scan(
			&rev.ID, &rev.WorkerID, &rev.UserID, &rev.UserName, &rev.BookingID,
			&rev.Rating, &rev.Comment, &photosJSON, &rev.Date, &rev.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		_ = json.Unmarshal(photosJSON, &rev.Photos)
		if rev.Photos == nil {
			rev.Photos = model.StringArray{}
		}
		result = append(result, rev)
	}
	return result, rows.Err()
}
