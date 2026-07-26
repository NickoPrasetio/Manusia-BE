package service_test

import (
	"context"
	"testing"

	"github.com/manusia/booking-service/internal/model"
	"github.com/manusia/booking-service/internal/service"
)

// ── stub repository — implements service.BookingRepository ─────────────────────

type stubBookingRepo struct {
	bookings  map[string]*model.Booking
	createErr error
}

func newStubBookingRepo() *stubBookingRepo {
	return &stubBookingRepo{bookings: map[string]*model.Booking{}}
}

func (s *stubBookingRepo) Create(_ context.Context, b *model.Booking) error {
	if s.createErr != nil {
		return s.createErr
	}
	b.ID = "booking-1"
	s.bookings[b.ID] = b
	return nil
}

func (s *stubBookingRepo) FindByCustomer(_ context.Context, customerID string) ([]model.Booking, error) {
	var result []model.Booking
	for _, b := range s.bookings {
		if b.CustomerID == customerID {
			result = append(result, *b)
		}
	}
	return result, nil
}

func (s *stubBookingRepo) FindByWorker(_ context.Context, workerID string) ([]model.Booking, error) {
	var result []model.Booking
	for _, b := range s.bookings {
		if b.WorkerID == workerID {
			result = append(result, *b)
		}
	}
	return result, nil
}

func (s *stubBookingRepo) FindByID(_ context.Context, id string) (*model.Booking, error) {
	b, ok := s.bookings[id]
	if !ok {
		return nil, errStubNotFound
	}
	return b, nil
}

func (s *stubBookingRepo) UpdateStatus(_ context.Context, id string, status model.BookingStatus) error {
	if b, ok := s.bookings[id]; ok {
		b.Status = status
	}
	return nil
}

func (s *stubBookingRepo) FindOpenNearby(_ context.Context, _, _, _ float64) ([]model.Booking, error) {
	var result []model.Booking
	for _, b := range s.bookings {
		if b.Status == model.StatusPending {
			result = append(result, *b)
		}
	}
	return result, nil
}

type stubErr struct{ msg string }

func (e *stubErr) Error() string { return e.msg }

var errStubNotFound = &stubErr{"not found"}

func addBooking(repo *stubBookingRepo, id, customerID, workerID string, status model.BookingStatus) {
	repo.bookings[id] = &model.Booking{ID: id, CustomerID: customerID, WorkerID: workerID, Status: status}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestBookingService_Create(t *testing.T) {
	ctx := context.Background()
	repo := newStubBookingRepo()
	svc := service.NewBookingService(repo)

	b, err := svc.Create(ctx, "cust-1", &model.CreateBookingRequest{
		WorkerID: "worker-1", CustomerName: "Andi", Address: "Jl. Mawar", City: "Jakarta",
		BookingDate: "2026-07-01", StartTime: "08:00", DurationDays: 1, PaymentMethod: "CASH",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Status != model.StatusPending {
		t.Errorf("expected status PENDING, got %s", b.Status)
	}
	if b.CustomerID != "cust-1" {
		t.Errorf("expected customerId cust-1, got %s", b.CustomerID)
	}
}

func TestBookingService_Confirm(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path - PENDING to CONFIRMED", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusPending)
		svc := service.NewBookingService(repo)

		b, err := svc.Confirm(ctx, "b1", "cust-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.Status != model.StatusConfirmed {
			t.Errorf("expected CONFIRMED, got %s", b.Status)
		}
	})

	t.Run("rejects non-PENDING booking", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusConfirmed)
		svc := service.NewBookingService(repo)

		_, err := svc.Confirm(ctx, "b1", "cust-1")
		if err != service.ErrNotPending {
			t.Errorf("expected ErrNotPending, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := newStubBookingRepo()
		svc := service.NewBookingService(repo)
		_, err := svc.Confirm(ctx, "missing", "cust-1")
		if err != service.ErrBookingNotFound {
			t.Errorf("expected ErrBookingNotFound, got %v", err)
		}
	})

	t.Run("forbidden - requester bukan pemilik booking", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusPending)
		svc := service.NewBookingService(repo)

		_, err := svc.Confirm(ctx, "b1", "cust-2")
		if err != service.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestBookingService_Complete(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path - CONFIRMED to COMPLETED", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusConfirmed)
		svc := service.NewBookingService(repo)

		b, err := svc.Complete(ctx, "b1", "cust-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.Status != model.StatusCompleted {
			t.Errorf("expected COMPLETED, got %s", b.Status)
		}
	})

	t.Run("rejects non-CONFIRMED booking", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusPending)
		svc := service.NewBookingService(repo)

		_, err := svc.Complete(ctx, "b1", "cust-1")
		if err != service.ErrNotConfirmed {
			t.Errorf("expected ErrNotConfirmed, got %v", err)
		}
	})

	t.Run("forbidden - requester bukan pemilik booking", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusConfirmed)
		svc := service.NewBookingService(repo)

		_, err := svc.Complete(ctx, "b1", "cust-2")
		if err != service.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestBookingService_Cancel(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path - cancels PENDING", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusPending)
		svc := service.NewBookingService(repo)

		b, err := svc.Cancel(ctx, "b1", "cust-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.Status != model.StatusCancelled {
			t.Errorf("expected CANCELLED, got %s", b.Status)
		}
	})

	t.Run("rejects cancelling COMPLETED booking", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusCompleted)
		svc := service.NewBookingService(repo)

		_, err := svc.Cancel(ctx, "b1", "cust-1")
		if err != service.ErrAlreadyFinalized {
			t.Errorf("expected ErrAlreadyFinalized, got %v", err)
		}
	})

	t.Run("forbidden - requester bukan pemilik booking", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusPending)
		svc := service.NewBookingService(repo)

		_, err := svc.Cancel(ctx, "b1", "cust-2")
		if err != service.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestBookingService_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("pemilik (customer) boleh lihat", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusPending)
		svc := service.NewBookingService(repo)

		b, err := svc.GetByID(ctx, "b1", "cust-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.ID != "b1" {
			t.Errorf("expected booking b1, got %s", b.ID)
		}
	})

	t.Run("worker terkait juga boleh lihat", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusPending)
		svc := service.NewBookingService(repo)

		_, err := svc.GetByID(ctx, "b1", "worker-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("forbidden - bukan pemilik maupun worker", func(t *testing.T) {
		repo := newStubBookingRepo()
		addBooking(repo, "b1", "cust-1", "worker-1", model.StatusPending)
		svc := service.NewBookingService(repo)

		_, err := svc.GetByID(ctx, "b1", "orang-lain")
		if err != service.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestBookingService_GetByCustomer(t *testing.T) {
	ctx := context.Background()

	t.Run("returns empty slice (not nil) when no bookings", func(t *testing.T) {
		repo := newStubBookingRepo()
		svc := service.NewBookingService(repo)
		bookings, err := svc.GetByCustomer(ctx, "cust-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bookings == nil {
			t.Error("expected empty slice, got nil")
		}
		if len(bookings) != 0 {
			t.Errorf("expected 0 bookings, got %d", len(bookings))
		}
	})
}
