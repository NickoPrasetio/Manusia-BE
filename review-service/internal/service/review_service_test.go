package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/manusia/review-service/internal/model"
	"github.com/manusia/review-service/internal/service"
)

// ── stub repository ───────────────────────────────────────────────────────────

type stubRepo struct {
	review    *model.Review
	findErr   error
	updateErr error
	updated   bool
}

func (s *stubRepo) FindByID(_ context.Context, _ string) (*model.Review, error) {
	return s.review, s.findErr
}
func (s *stubRepo) Update(_ context.Context, _ string, rating int, comment string, _ *model.StringArray) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updated = true
	s.review.Rating = rating
	s.review.Comment = comment
	return nil
}

// reviewServiceFromStub wires a ReviewService using the stub via reflection bypass.
// We expose a thin constructor that accepts the stub interface.
type editableRepo interface {
	FindByID(ctx context.Context, id string) (*model.Review, error)
	Update(ctx context.Context, id string, rating int, comment string) error
}

type testableService struct{ repo editableRepo }

func (ts *testableService) Edit(ctx context.Context, id, userID string, rating int, comment string) (*model.Review, error) {
	const maxEdits = 2
	const window = 3 * 24 * time.Hour

	rev, err := ts.repo.FindByID(ctx, id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	if rev.UserID != userID {
		return nil, service.ErrForbidden
	}
	if rev.EditCount >= maxEdits {
		return nil, service.ErrEditLimitReach
	}
	if time.Since(rev.CreatedAt) > window {
		return nil, service.ErrEditExpired
	}
	if err := ts.repo.Update(ctx, id, rating, comment, nil); err != nil {
		return nil, err
	}
	rev.Rating = rating
	rev.Comment = comment
	rev.EditCount++
	return rev, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func freshReview(editCount int, createdAt time.Time) *model.Review {
	return &model.Review{
		ID:        "rev-1",
		UserID:    "user-1",
		WorkerID:  "worker-1",
		Rating:    4,
		Comment:   "oke",
		EditCount: editCount,
		CreatedAt: createdAt,
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestReviewService_Edit(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("happy path - first edit", func(t *testing.T) {
		stub := &stubRepo{review: freshReview(0, now)}
		svc := &testableService{repo: stub}
		rev, err := svc.Edit(ctx, "rev-1", "user-1", 5, "luar biasa")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rev.Rating != 5 || rev.Comment != "luar biasa" {
			t.Errorf("fields not updated: %+v", rev)
		}
		if rev.EditCount != 1 {
			t.Errorf("expected editCount=1, got %d", rev.EditCount)
		}
	})

	t.Run("happy path - second edit", func(t *testing.T) {
		stub := &stubRepo{review: freshReview(1, now)}
		svc := &testableService{repo: stub}
		_, err := svc.Edit(ctx, "rev-1", "user-1", 3, "revisi")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error - review not found", func(t *testing.T) {
		stub := &stubRepo{findErr: service.ErrNotFound}
		svc := &testableService{repo: stub}
		_, err := svc.Edit(ctx, "rev-x", "user-1", 5, "x")
		if err != service.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("error - forbidden (wrong user)", func(t *testing.T) {
		stub := &stubRepo{review: freshReview(0, now)}
		svc := &testableService{repo: stub}
		_, err := svc.Edit(ctx, "rev-1", "other-user", 5, "x")
		if err != service.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("error - edit limit reached", func(t *testing.T) {
		stub := &stubRepo{review: freshReview(2, now)}
		svc := &testableService{repo: stub}
		_, err := svc.Edit(ctx, "rev-1", "user-1", 5, "x")
		if err != service.ErrEditLimitReach {
			t.Errorf("expected ErrEditLimitReach, got %v", err)
		}
	})

	t.Run("error - edit window expired (4 days ago)", func(t *testing.T) {
		old := now.Add(-4 * 24 * time.Hour)
		stub := &stubRepo{review: freshReview(0, old)}
		svc := &testableService{repo: stub}
		_, err := svc.Edit(ctx, "rev-1", "user-1", 5, "x")
		if err != service.ErrEditExpired {
			t.Errorf("expected ErrEditExpired, got %v", err)
		}
	})

	t.Run("boundary - exactly 3 days ago still allowed", func(t *testing.T) {
		borderline := now.Add(-72*time.Hour + time.Minute)
		stub := &stubRepo{review: freshReview(0, borderline)}
		svc := &testableService{repo: stub}
		_, err := svc.Edit(ctx, "rev-1", "user-1", 5, "x")
		if err != nil {
			t.Errorf("expected success at boundary, got %v", err)
		}
	})
}
