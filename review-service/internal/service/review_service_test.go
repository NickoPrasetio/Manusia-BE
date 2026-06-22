package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/manusia/review-service/internal/model"
	"github.com/manusia/review-service/internal/service"
)

// ── stub repository — implements service.ReviewRepository ──────────────────────

type stubRepo struct {
	reviewsByWorker map[string][]model.Review
	reviewsByUser   map[string][]model.Review
	byID            map[string]*model.Review

	findErr   error
	updateErr error
	createErr error
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		reviewsByWorker: map[string][]model.Review{},
		reviewsByUser:   map[string][]model.Review{},
		byID:            map[string]*model.Review{},
	}
}

func (s *stubRepo) FindByID(_ context.Context, id string) (*model.Review, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	rev, ok := s.byID[id]
	if !ok {
		return nil, errStubNotFound
	}
	return rev, nil
}

func (s *stubRepo) Update(_ context.Context, id string, rating int, comment string, _ *model.StringArray) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if rev, ok := s.byID[id]; ok {
		rev.Rating = rating
		rev.Comment = comment
	}
	return nil
}

func (s *stubRepo) FindByWorker(_ context.Context, workerID string) ([]model.Review, error) {
	return s.reviewsByWorker[workerID], nil
}

func (s *stubRepo) FindPage(_ context.Context, workerID string, page, limit int) ([]model.Review, int, float64, []model.RatingDist, error) {
	all := s.reviewsByWorker[workerID]
	total := len(all)
	offset := page * limit
	if offset >= total {
		return []model.Review{}, total, 0, nil, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, 4.5, nil, nil
}

func (s *stubRepo) FindGivenByUser(_ context.Context, userID string, page, limit int) ([]model.Review, int, error) {
	all := s.reviewsByUser[userID]
	total := len(all)
	offset := page * limit
	if offset >= total {
		return []model.Review{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (s *stubRepo) Create(_ context.Context, rev *model.Review) error {
	if s.createErr != nil {
		return s.createErr
	}
	rev.ID = "rev-new"
	s.byID[rev.ID] = rev
	s.reviewsByWorker[rev.WorkerID] = append(s.reviewsByWorker[rev.WorkerID], *rev)
	s.reviewsByUser[rev.UserID] = append(s.reviewsByUser[rev.UserID], *rev)
	return nil
}

func (s *stubRepo) AverageRating(_ context.Context, _ string) (float64, int, error) {
	return 4.5, 1, nil
}

type stubErr struct{ msg string }

func (e *stubErr) Error() string { return e.msg }

var errStubNotFound = &stubErr{"not found"}

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

// ── Edit tests ───────────────────────────────────────────────────────────────

func TestReviewService_Edit(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("happy path - first edit", func(t *testing.T) {
		repo := newStubRepo()
		repo.byID["rev-1"] = freshReview(0, now)
		svc := service.NewReviewService(repo, "")

		rev, err := svc.Edit(ctx, "rev-1", "user-1", 5, "luar biasa", nil)
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
		repo := newStubRepo()
		repo.byID["rev-1"] = freshReview(1, now)
		svc := service.NewReviewService(repo, "")

		_, err := svc.Edit(ctx, "rev-1", "user-1", 3, "revisi", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error - review not found", func(t *testing.T) {
		repo := newStubRepo()
		svc := service.NewReviewService(repo, "")
		_, err := svc.Edit(ctx, "rev-x", "user-1", 5, "x", nil)
		if err != service.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("error - forbidden (wrong user)", func(t *testing.T) {
		repo := newStubRepo()
		repo.byID["rev-1"] = freshReview(0, now)
		svc := service.NewReviewService(repo, "")
		_, err := svc.Edit(ctx, "rev-1", "other-user", 5, "x", nil)
		if err != service.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("error - edit limit reached", func(t *testing.T) {
		repo := newStubRepo()
		repo.byID["rev-1"] = freshReview(2, now)
		svc := service.NewReviewService(repo, "")
		_, err := svc.Edit(ctx, "rev-1", "user-1", 5, "x", nil)
		if err != service.ErrEditLimitReach {
			t.Errorf("expected ErrEditLimitReach, got %v", err)
		}
	})

	t.Run("error - edit window expired (4 days ago)", func(t *testing.T) {
		repo := newStubRepo()
		repo.byID["rev-1"] = freshReview(0, now.Add(-4*24*time.Hour))
		svc := service.NewReviewService(repo, "")
		_, err := svc.Edit(ctx, "rev-1", "user-1", 5, "x", nil)
		if err != service.ErrEditExpired {
			t.Errorf("expected ErrEditExpired, got %v", err)
		}
	})

	t.Run("boundary - exactly within 3 days still allowed", func(t *testing.T) {
		repo := newStubRepo()
		repo.byID["rev-1"] = freshReview(0, now.Add(-72*time.Hour+time.Minute))
		svc := service.NewReviewService(repo, "")
		_, err := svc.Edit(ctx, "rev-1", "user-1", 5, "x", nil)
		if err != nil {
			t.Errorf("expected success at boundary, got %v", err)
		}
	})
}

// ── GetByWorker / GetByWorkerPage / GetGivenByUser tests ───────────────────────

func TestReviewService_GetByWorker(t *testing.T) {
	ctx := context.Background()

	t.Run("returns empty slice (not nil) when no reviews", func(t *testing.T) {
		repo := newStubRepo()
		svc := service.NewReviewService(repo, "")
		reviews, err := svc.GetByWorker(ctx, "worker-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reviews == nil || len(reviews) != 0 {
			t.Errorf("expected empty non-nil slice, got %v", reviews)
		}
	})
}

func TestReviewService_GetByWorkerPage(t *testing.T) {
	ctx := context.Background()
	repo := newStubRepo()
	repo.reviewsByWorker["worker-1"] = []model.Review{{ID: "r1"}, {ID: "r2"}, {ID: "r3"}}
	svc := service.NewReviewService(repo, "")

	page, err := svc.GetByWorkerPage(ctx, "worker-1", 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 3 || len(page.Reviews) != 2 || page.Last {
		t.Errorf("unexpected page: %+v", page)
	}

	page2, err := svc.GetByWorkerPage(ctx, "worker-1", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !page2.Last || len(page2.Reviews) != 1 {
		t.Errorf("expected last page with 1 review, got %+v", page2)
	}
}

func TestReviewService_GetGivenByUser(t *testing.T) {
	ctx := context.Background()
	repo := newStubRepo()
	repo.reviewsByUser["user-1"] = []model.Review{{ID: "r1"}, {ID: "r2"}}
	svc := service.NewReviewService(repo, "")

	page, err := svc.GetGivenByUser(ctx, "user-1", 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 2 || !page.Last {
		t.Errorf("unexpected page: %+v", page)
	}
}

// ── CreateReview tests ──────────────────────────────────────────────────────

func TestReviewService_CreateReview(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		repo := newStubRepo()
		svc := service.NewReviewService(repo, "")
		rev := &model.Review{WorkerID: "worker-1", UserID: "user-1", Rating: 5}

		if err := svc.CreateReview(ctx, rev); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rev.ID == "" {
			t.Error("expected review ID to be populated after create")
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := newStubRepo()
		repo.createErr = errStubNotFound
		svc := service.NewReviewService(repo, "")
		err := svc.CreateReview(ctx, &model.Review{WorkerID: "worker-1"})
		if err == nil {
			t.Error("expected error to propagate")
		}
	})
}
