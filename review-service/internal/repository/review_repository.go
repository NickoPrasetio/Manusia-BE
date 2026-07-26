package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/review-service/internal/model"
)

const selectColumns = `id, worker_id, worker_name, worker_avatar, user_id, user_name, booking_id,
	rating, comment, photos, date, edit_count, created_at`

const qualifiedSelectColumns = `reviews.id, reviews.worker_id, reviews.worker_name, reviews.worker_avatar,
	reviews.user_id, reviews.user_name, reviews.booking_id, reviews.rating, reviews.comment, reviews.photos,
	reviews.date, reviews.edit_count, reviews.created_at`

type ReviewRepository struct {
	db *pgxpool.Pool
}

func NewReviewRepository(db *pgxpool.Pool) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS reviews (
			id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			worker_id     TEXT NOT NULL,
			worker_name   TEXT NOT NULL DEFAULT '',
			worker_avatar TEXT NOT NULL DEFAULT '',
			user_id       TEXT NOT NULL,
			user_name     TEXT NOT NULL,
			booking_id    TEXT NOT NULL DEFAULT '',
			rating        INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
			comment       TEXT NOT NULL DEFAULT '',
			photos        JSONB NOT NULL DEFAULT '[]',
			date          TEXT NOT NULL,
			edit_count    INT NOT NULL DEFAULT 0,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE reviews ADD COLUMN IF NOT EXISTS edit_count INT NOT NULL DEFAULT 0;
		ALTER TABLE reviews ADD COLUMN IF NOT EXISTS worker_name TEXT NOT NULL DEFAULT '';
		ALTER TABLE reviews ADD COLUMN IF NOT EXISTS worker_avatar TEXT NOT NULL DEFAULT '';
	`)
	return err
}

func (r *ReviewRepository) Create(ctx context.Context, rev *model.Review) error {
	photos, _ := json.Marshal(rev.Photos)
	return r.db.QueryRow(ctx, `
		INSERT INTO reviews (worker_id, worker_name, worker_avatar, user_id, user_name, booking_id, rating, comment, photos, date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, edit_count, created_at
	`, rev.WorkerID, rev.WorkerName, rev.WorkerAvatar, rev.UserID, rev.UserName, rev.BookingID, rev.Rating, rev.Comment, string(photos), rev.Date).
		Scan(&rev.ID, &rev.EditCount, &rev.CreatedAt)
}

func (r *ReviewRepository) FindByID(ctx context.Context, id string) (*model.Review, error) {
	var rev model.Review
	var photosJSON []byte
	err := r.db.QueryRow(ctx, `SELECT `+selectColumns+` FROM reviews WHERE id=$1`, id).Scan(
		&rev.ID, &rev.WorkerID, &rev.WorkerName, &rev.WorkerAvatar, &rev.UserID, &rev.UserName, &rev.BookingID,
		&rev.Rating, &rev.Comment, &photosJSON, &rev.Date, &rev.EditCount, &rev.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(photosJSON, &rev.Photos)
	if rev.Photos == nil {
		rev.Photos = model.StringArray{}
	}
	return &rev, nil
}

// Update modifies rating, comment, and optionally photos (nil = keep existing).
func (r *ReviewRepository) Update(ctx context.Context, id string, rating int, comment string, photos *model.StringArray) error {
	if photos == nil {
		_, err := r.db.Exec(ctx, `
			UPDATE reviews SET rating=$1, comment=$2, edit_count=edit_count+1 WHERE id=$3
		`, rating, comment, id)
		return err
	}
	photosJSON, _ := json.Marshal(photos)
	_, err := r.db.Exec(ctx, `
		UPDATE reviews SET rating=$1, comment=$2, photos=$3, edit_count=edit_count+1 WHERE id=$4
	`, rating, comment, string(photosJSON), id)
	return err
}

func (r *ReviewRepository) FindByWorker(ctx context.Context, workerID string) ([]model.Review, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+selectColumns+`
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

	// paginated reviews — LEFT JOIN review_appeals so the caller (my-reviews
	// page) can show appeal status inline without extra requests.
	offset := page * limit
	reviewRows, err := r.db.Query(ctx,
		`SELECT `+qualifiedSelectColumns+`, ra.id, ra.status, NULLIF(ra.ai_verdict, ''), NULLIF(ra.ai_reasoning, '')
		 FROM reviews
		 LEFT JOIN review_appeals ra ON ra.review_id = reviews.id
		 WHERE reviews.worker_id=$1
		 ORDER BY reviews.created_at DESC, reviews.id DESC
		 LIMIT $2 OFFSET $3`,
		workerID, limit, offset)
	if err != nil {
		return nil, 0, 0, nil, err
	}
	defer reviewRows.Close()

	var reviews []model.Review
	for reviewRows.Next() {
		var rev model.Review
		var photosJSON []byte
		if err := reviewRows.Scan(
			&rev.ID, &rev.WorkerID, &rev.WorkerName, &rev.WorkerAvatar, &rev.UserID, &rev.UserName, &rev.BookingID,
			&rev.Rating, &rev.Comment, &photosJSON, &rev.Date, &rev.EditCount, &rev.CreatedAt,
			&rev.AppealID, &rev.AppealStatus, &rev.AIVerdict, &rev.AIReasoning,
		); err != nil {
			return nil, 0, 0, nil, fmt.Errorf("scan review with appeal: %w", err)
		}
		_ = json.Unmarshal(photosJSON, &rev.Photos)
		if rev.Photos == nil {
			rev.Photos = model.StringArray{}
		}
		reviews = append(reviews, rev)
	}
	if err := reviewRows.Err(); err != nil {
		return nil, 0, 0, nil, err
	}
	return reviews, total, avg, dist, nil
}

func (r *ReviewRepository) FindGivenByUser(ctx context.Context, userID string, page, limit int) ([]model.Review, int, error) {
	var total int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM reviews WHERE user_id=$1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := page * limit
	rows, err := r.db.Query(ctx,
		`SELECT `+selectColumns+`
		 FROM reviews WHERE user_id=$1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	reviews, err := scanReviews(rows)
	return reviews, total, err
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
			&rev.ID, &rev.WorkerID, &rev.WorkerName, &rev.WorkerAvatar, &rev.UserID, &rev.UserName, &rev.BookingID,
			&rev.Rating, &rev.Comment, &photosJSON, &rev.Date, &rev.EditCount, &rev.CreatedAt,
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
