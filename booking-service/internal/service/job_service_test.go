package service_test

import (
	"context"
	"testing"

	"github.com/manusia/booking-service/internal/model"
	"github.com/manusia/booking-service/internal/service"
)

// ── stub repository — implements service.JobRepository ─────────────────────────

type stubJobRepo struct {
	jobs map[string]*model.Job
}

func newStubJobRepo() *stubJobRepo {
	return &stubJobRepo{jobs: map[string]*model.Job{}}
}

func (s *stubJobRepo) Create(_ context.Context, j *model.Job) error {
	j.ID = "job-1"
	s.jobs[j.ID] = j
	return nil
}

func (s *stubJobRepo) FindByID(_ context.Context, id string) (*model.Job, error) {
	j, ok := s.jobs[id]
	if !ok {
		return nil, errStubNotFound
	}
	return j, nil
}

func (s *stubJobRepo) FindByCustomer(_ context.Context, customerID string) ([]model.Job, error) {
	var result []model.Job
	for _, j := range s.jobs {
		if j.CustomerID == customerID {
			result = append(result, *j)
		}
	}
	return result, nil
}

func (s *stubJobRepo) UpdateStatus(_ context.Context, id string, status model.JobStatus) error {
	if j, ok := s.jobs[id]; ok {
		j.Status = status
	}
	return nil
}

func (s *stubJobRepo) FindNearby(_ context.Context, _, _, _ float64, _ string, _, _ int) ([]model.Job, int64, error) {
	var result []model.Job
	for _, j := range s.jobs {
		if j.Status == model.JobStatusOpen {
			result = append(result, *j)
		}
	}
	return result, int64(len(result)), nil
}

func addJob(repo *stubJobRepo, id, customerID string, status model.JobStatus) {
	repo.jobs[id] = &model.Job{ID: id, CustomerID: customerID, Title: "Bersihkan rumah", Status: status, DurationDays: 1}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestJobService_Create(t *testing.T) {
	ctx := context.Background()
	repo := newStubJobRepo()
	svc := service.NewJobService(repo, newStubBookingRepo())

	j, err := svc.Create(ctx, "cust-1", &model.CreateJobRequest{
		CustomerName: "Andi", Title: "Bersihkan taman", Description: "desc",
		BudgetPerDay: 100000, DurationDays: 2, City: "Bandung", Category: model.CategoryTask,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j.Status != model.JobStatusOpen {
		t.Errorf("expected OPEN, got %s", j.Status)
	}
	if j.TodoList == nil {
		t.Error("expected TodoList initialized to empty slice, got nil")
	}
}

func TestJobService_Close(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path - owner closes own job", func(t *testing.T) {
		repo := newStubJobRepo()
		addJob(repo, "j1", "cust-1", model.JobStatusOpen)
		svc := service.NewJobService(repo, newStubBookingRepo())

		j, err := svc.Close(ctx, "j1", "cust-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if j.Status != model.JobStatusClosed {
			t.Errorf("expected CLOSED, got %s", j.Status)
		}
	})

	t.Run("rejects non-owner", func(t *testing.T) {
		repo := newStubJobRepo()
		addJob(repo, "j1", "cust-1", model.JobStatusOpen)
		svc := service.NewJobService(repo, newStubBookingRepo())

		_, err := svc.Close(ctx, "j1", "stranger")
		if err != service.ErrJobForbidden {
			t.Errorf("expected ErrJobForbidden, got %v", err)
		}
	})

	t.Run("rejects already-closed job", func(t *testing.T) {
		repo := newStubJobRepo()
		addJob(repo, "j1", "cust-1", model.JobStatusClosed)
		svc := service.NewJobService(repo, newStubBookingRepo())

		_, err := svc.Close(ctx, "j1", "cust-1")
		if err != service.ErrJobAlreadyClosed {
			t.Errorf("expected ErrJobAlreadyClosed, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := newStubJobRepo()
		svc := service.NewJobService(repo, newStubBookingRepo())
		_, err := svc.Close(ctx, "missing", "cust-1")
		if err != service.ErrJobNotFound {
			t.Errorf("expected ErrJobNotFound, got %v", err)
		}
	})
}

func TestJobService_ApplyToJob(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path - creates booking", func(t *testing.T) {
		jobRepo := newStubJobRepo()
		addJob(jobRepo, "j1", "cust-1", model.JobStatusOpen)
		bookingRepo := newStubBookingRepo()
		svc := service.NewJobService(jobRepo, bookingRepo)

		b, err := svc.ApplyToJob(ctx, service.ApplyToJobInput{
			JobID: "j1", WorkerID: "worker-1", WorkerName: "Budi",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.CustomerID != "cust-1" || b.WorkerID != "worker-1" {
			t.Errorf("unexpected booking: %+v", b)
		}
		if b.JobID != "j1" {
			t.Errorf("expected jobId j1, got %s", b.JobID)
		}
	})

	t.Run("rejects applying to own job", func(t *testing.T) {
		jobRepo := newStubJobRepo()
		addJob(jobRepo, "j1", "cust-1", model.JobStatusOpen)
		svc := service.NewJobService(jobRepo, newStubBookingRepo())

		_, err := svc.ApplyToJob(ctx, service.ApplyToJobInput{JobID: "j1", WorkerID: "cust-1"})
		if err != service.ErrJobSelfApply {
			t.Errorf("expected ErrJobSelfApply, got %v", err)
		}
	})

	t.Run("rejects applying to closed job", func(t *testing.T) {
		jobRepo := newStubJobRepo()
		addJob(jobRepo, "j1", "cust-1", model.JobStatusClosed)
		svc := service.NewJobService(jobRepo, newStubBookingRepo())

		_, err := svc.ApplyToJob(ctx, service.ApplyToJobInput{JobID: "j1", WorkerID: "worker-1"})
		if err != service.ErrJobNotOpen {
			t.Errorf("expected ErrJobNotOpen, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		jobRepo := newStubJobRepo()
		svc := service.NewJobService(jobRepo, newStubBookingRepo())
		_, err := svc.ApplyToJob(ctx, service.ApplyToJobInput{JobID: "missing", WorkerID: "worker-1"})
		if err != service.ErrJobNotFound {
			t.Errorf("expected ErrJobNotFound, got %v", err)
		}
	})
}
