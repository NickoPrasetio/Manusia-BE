package service

import (
	"context"
	"errors"

	"github.com/manusia/booking-service/internal/model"
)

var (
	ErrBookingNotFound  = errors.New("booking tidak ditemukan")
	ErrNotPending       = errors.New("hanya booking PENDING yang bisa dikonfirmasi")
	ErrNotConfirmed     = errors.New("hanya booking CONFIRMED yang bisa diselesaikan")
	ErrAlreadyFinalized = errors.New("booking tidak bisa dibatalkan")
	ErrForbidden        = errors.New("kamu bukan pemilik booking ini")
)

// BookingRepository is the persistence contract BookingService depends on —
// satisfied implicitly by *repository.BookingRepository.
type BookingRepository interface {
	Create(ctx context.Context, b *model.Booking) error
	FindByCustomer(ctx context.Context, customerID string) ([]model.Booking, error)
	FindByWorker(ctx context.Context, workerID string) ([]model.Booking, error)
	FindByID(ctx context.Context, id string) (*model.Booking, error)
	UpdateStatus(ctx context.Context, id string, status model.BookingStatus) error
	FindOpenNearby(ctx context.Context, lat, lon, radiusKm float64) ([]model.Booking, error)
}

type BookingService struct {
	repo BookingRepository
}

func NewBookingService(repo BookingRepository) *BookingService {
	return &BookingService{repo: repo}
}

func (s *BookingService) Create(ctx context.Context, customerID string, req *model.CreateBookingRequest) (*model.Booking, error) {
	b := &model.Booking{
		WorkerID:      req.WorkerID,
		WorkerName:    req.WorkerName,
		WorkerAvatar:  req.WorkerAvatar,
		CustomerID:    customerID,
		CustomerName:  req.CustomerName,
		Address:       req.Address,
		City:          req.City,
		Latitude:      req.Latitude,
		Longitude:     req.Longitude,
		BookingDate:   req.BookingDate,
		StartTime:     req.StartTime,
		DurationDays:  req.DurationDays,
		PaymentMethod: req.PaymentMethod,
		Notes:         req.Notes,
		Status:        model.StatusPending,
	}
	if err := s.repo.Create(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *BookingService) GetByCustomer(ctx context.Context, customerID string) ([]model.Booking, error) {
	bookings, err := s.repo.FindByCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if bookings == nil {
		bookings = []model.Booking{}
	}
	return bookings, nil
}

func (s *BookingService) GetByWorker(ctx context.Context, workerID string) ([]model.Booking, error) {
	bookings, err := s.repo.FindByWorker(ctx, workerID)
	if err != nil {
		return nil, err
	}
	if bookings == nil {
		bookings = []model.Booking{}
	}
	return bookings, nil
}

func (s *BookingService) GetByID(ctx context.Context, id, requesterID string) (*model.Booking, error) {
	b, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrBookingNotFound
	}
	if b.CustomerID != requesterID && b.WorkerID != requesterID {
		return nil, ErrForbidden
	}
	return b, nil
}

func (s *BookingService) Confirm(ctx context.Context, id, requesterID string) (*model.Booking, error) {
	b, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrBookingNotFound
	}
	if b.CustomerID != requesterID {
		return nil, ErrForbidden
	}
	if b.Status != model.StatusPending {
		return nil, ErrNotPending
	}
	if err := s.repo.UpdateStatus(ctx, id, model.StatusConfirmed); err != nil {
		return nil, err
	}
	b.Status = model.StatusConfirmed
	return b, nil
}

func (s *BookingService) Complete(ctx context.Context, id, requesterID string) (*model.Booking, error) {
	b, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrBookingNotFound
	}
	if b.CustomerID != requesterID {
		return nil, ErrForbidden
	}
	if b.Status != model.StatusConfirmed {
		return nil, ErrNotConfirmed
	}
	if err := s.repo.UpdateStatus(ctx, id, model.StatusCompleted); err != nil {
		return nil, err
	}
	b.Status = model.StatusCompleted
	return b, nil
}

func (s *BookingService) Cancel(ctx context.Context, id, requesterID string) (*model.Booking, error) {
	b, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrBookingNotFound
	}
	if b.CustomerID != requesterID {
		return nil, ErrForbidden
	}
	if b.Status == model.StatusCompleted || b.Status == model.StatusCancelled {
		return nil, ErrAlreadyFinalized
	}
	if err := s.repo.UpdateStatus(ctx, id, model.StatusCancelled); err != nil {
		return nil, err
	}
	b.Status = model.StatusCancelled
	return b, nil
}

func (s *BookingService) GetOpenNearby(ctx context.Context, lat, lon, radiusKm float64) ([]model.Booking, error) {
	bookings, err := s.repo.FindOpenNearby(ctx, lat, lon, radiusKm)
	if err != nil {
		return nil, err
	}
	if bookings == nil {
		bookings = []model.Booking{}
	}
	return bookings, nil
}
