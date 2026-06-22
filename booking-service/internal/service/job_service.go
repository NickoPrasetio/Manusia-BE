package service

import (
	"context"
	"errors"

	"github.com/manusia/booking-service/internal/model"
)

var (
	ErrJobNotFound      = errors.New("job tidak ditemukan")
	ErrJobNotOpen       = errors.New("job sudah tidak tersedia")
	ErrJobSelfApply     = errors.New("tidak bisa melamar job milik sendiri")
	ErrJobForbidden     = errors.New("bukan pemilik job ini")
	ErrJobAlreadyClosed = errors.New("job sudah ditutup")
)

// JobRepository is the persistence contract JobService depends on —
// satisfied implicitly by *repository.JobRepository.
type JobRepository interface {
	Create(ctx context.Context, j *model.Job) error
	FindByID(ctx context.Context, id string) (*model.Job, error)
	FindByCustomer(ctx context.Context, customerID string) ([]model.Job, error)
	UpdateStatus(ctx context.Context, id string, status model.JobStatus) error
	FindNearby(ctx context.Context, lat, lon, radiusKm float64, category string, limit, offset int) ([]model.Job, int64, error)
}

type JobService struct {
	repo        JobRepository
	bookingRepo BookingRepository
}

func NewJobService(repo JobRepository, bookingRepo BookingRepository) *JobService {
	return &JobService{repo: repo, bookingRepo: bookingRepo}
}

func (s *JobService) Create(ctx context.Context, customerID string, req *model.CreateJobRequest) (*model.Job, error) {
	j := &model.Job{
		CustomerID:   customerID,
		CustomerName: req.CustomerName,
		Title:        req.Title,
		Description:  req.Description,
		BudgetPerDay: req.BudgetPerDay,
		TodoList:     req.TodoList,
		DurationDays: req.DurationDays,
		City:         req.City,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		Category:     req.Category,
		Status:       model.JobStatusOpen,
	}
	if j.TodoList == nil {
		j.TodoList = []string{}
	}
	if err := s.repo.Create(ctx, j); err != nil {
		return nil, err
	}
	return j, nil
}

func (s *JobService) GetByCustomer(ctx context.Context, customerID string) ([]model.Job, error) {
	jobs, err := s.repo.FindByCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if jobs == nil {
		jobs = []model.Job{}
	}
	return jobs, nil
}

func (s *JobService) GetByID(ctx context.Context, id string) (*model.Job, error) {
	j, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrJobNotFound
	}
	return j, nil
}

func (s *JobService) Close(ctx context.Context, id, customerID string) (*model.Job, error) {
	j, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrJobNotFound
	}
	if j.CustomerID != customerID {
		return nil, ErrJobForbidden
	}
	if j.Status == model.JobStatusClosed {
		return nil, ErrJobAlreadyClosed
	}
	if err := s.repo.UpdateStatus(ctx, id, model.JobStatusClosed); err != nil {
		return nil, err
	}
	j.Status = model.JobStatusClosed
	return j, nil
}

func (s *JobService) GetNearby(ctx context.Context, lat, lon, radiusKm float64, category string, limit, offset int) ([]model.Job, int64, error) {
	jobs, total, err := s.repo.FindNearby(ctx, lat, lon, radiusKm, category, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	if jobs == nil {
		jobs = []model.Job{}
	}
	return jobs, total, nil
}

// ApplyToJobInput carries everything needed to convert a job application into a Booking.
type ApplyToJobInput struct {
	JobID        string
	WorkerID     string
	WorkerName   string
	WorkerAvatar string
	Notes        string
}

func (s *JobService) ApplyToJob(ctx context.Context, input ApplyToJobInput) (*model.Booking, error) {
	job, err := s.repo.FindByID(ctx, input.JobID)
	if err != nil {
		return nil, ErrJobNotFound
	}
	if job.Status != model.JobStatusOpen {
		return nil, ErrJobNotOpen
	}
	if job.CustomerID == input.WorkerID {
		return nil, ErrJobSelfApply
	}

	b := &model.Booking{
		WorkerID:      input.WorkerID,
		WorkerName:    input.WorkerName,
		WorkerAvatar:  input.WorkerAvatar,
		CustomerID:    job.CustomerID,
		CustomerName:  job.CustomerName,
		Address:       job.City,
		City:          job.City,
		Latitude:      job.Latitude,
		Longitude:     job.Longitude,
		BookingDate:   "",
		StartTime:     "08:00",
		DurationDays:  job.DurationDays,
		PaymentMethod: "CASH",
		Notes:         input.Notes,
		JobID:         input.JobID,
		JobTitle:      job.Title,
		Status:        model.StatusPending,
	}
	if err := s.bookingRepo.Create(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}
