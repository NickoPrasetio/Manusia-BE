package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manusia/review-service/internal/model"
)

const appealColumns = `id, review_id, appellant_id, appellant_name, reviewer_id, reviewer_name,
	appellant_comment, reviewer_comment, status, ai_verdict, ai_reasoning, conversation_id,
	deadline_at, reviewer_responded_at, resolved_at, created_at`

type AppealRepository struct {
	db *pgxpool.Pool
}

func NewAppealRepository(db *pgxpool.Pool) *AppealRepository {
	return &AppealRepository{db: db}
}

func (r *AppealRepository) Migrate(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS review_appeals (
			id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			review_id             UUID NOT NULL UNIQUE REFERENCES reviews(id) ON DELETE CASCADE,
			appellant_id          TEXT NOT NULL,
			appellant_name        TEXT NOT NULL DEFAULT '',
			reviewer_id           TEXT NOT NULL,
			reviewer_name         TEXT NOT NULL DEFAULT '',
			appellant_comment     TEXT NOT NULL,
			reviewer_comment      TEXT NOT NULL DEFAULT '',
			status                TEXT NOT NULL DEFAULT 'pending',
			ai_verdict            TEXT NOT NULL DEFAULT '',
			ai_reasoning          TEXT NOT NULL DEFAULT '',
			conversation_id       TEXT NOT NULL DEFAULT '',
			deadline_at           TIMESTAMPTZ NOT NULL,
			reviewer_responded_at TIMESTAMPTZ,
			resolved_at           TIMESTAMPTZ,
			created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	return err
}

func (r *AppealRepository) Create(ctx context.Context, a *model.Appeal) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO review_appeals (review_id, appellant_id, appellant_name, reviewer_id, reviewer_name, appellant_comment, deadline_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, status, created_at
	`, a.ReviewID, a.AppellantID, a.AppellantName, a.ReviewerID, a.ReviewerName, a.AppellantComment, a.DeadlineAt).
		Scan(&a.ID, &a.Status, &a.CreatedAt)
}

func (r *AppealRepository) GetByID(ctx context.Context, id string) (*model.Appeal, error) {
	var a model.Appeal
	err := r.db.QueryRow(ctx, `SELECT `+appealColumns+` FROM review_appeals WHERE id=$1`, id).Scan(
		&a.ID, &a.ReviewID, &a.AppellantID, &a.AppellantName, &a.ReviewerID, &a.ReviewerName,
		&a.AppellantComment, &a.ReviewerComment, &a.Status, &a.AIVerdict, &a.AIReasoning, &a.ConversationID,
		&a.DeadlineAt, &a.ReviewerRespondedAt, &a.ResolvedAt, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AppealRepository) GetByReviewID(ctx context.Context, reviewID string) (*model.Appeal, error) {
	var a model.Appeal
	err := r.db.QueryRow(ctx, `SELECT `+appealColumns+` FROM review_appeals WHERE review_id=$1`, reviewID).Scan(
		&a.ID, &a.ReviewID, &a.AppellantID, &a.AppellantName, &a.ReviewerID, &a.ReviewerName,
		&a.AppellantComment, &a.ReviewerComment, &a.Status, &a.AIVerdict, &a.AIReasoning, &a.ConversationID,
		&a.DeadlineAt, &a.ReviewerRespondedAt, &a.ResolvedAt, &a.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (r *AppealRepository) SetConversationID(ctx context.Context, id, conversationID string) error {
	_, err := r.db.Exec(ctx, `UPDATE review_appeals SET conversation_id=$1 WHERE id=$2`, conversationID, id)
	return err
}

func (r *AppealRepository) SetReviewerComment(ctx context.Context, id, comment string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE review_appeals SET reviewer_comment=$1, reviewer_responded_at=NOW() WHERE id=$2
	`, comment, id)
	return err
}

func (r *AppealRepository) SetVerdict(ctx context.Context, id, verdict, reasoning string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE review_appeals SET ai_verdict=$1, ai_reasoning=$2, status=$3, resolved_at=NOW() WHERE id=$4
	`, verdict, reasoning, model.AppealStatusResolved, id)
	return err
}

// ListDueForModeration returns pending appeals ready for AI moderation:
// either the reviewer has already responded, or the deadline has passed.
func (r *AppealRepository) ListDueForModeration(ctx context.Context, now time.Time) ([]model.Appeal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+appealColumns+`
		FROM review_appeals
		WHERE status=$1 AND (reviewer_comment <> '' OR deadline_at <= $2)
	`, model.AppealStatusPending, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.Appeal
	for rows.Next() {
		var a model.Appeal
		if err := rows.Scan(
			&a.ID, &a.ReviewID, &a.AppellantID, &a.AppellantName, &a.ReviewerID, &a.ReviewerName,
			&a.AppellantComment, &a.ReviewerComment, &a.Status, &a.AIVerdict, &a.AIReasoning, &a.ConversationID,
			&a.DeadlineAt, &a.ReviewerRespondedAt, &a.ResolvedAt, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
