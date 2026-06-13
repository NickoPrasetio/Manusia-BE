package model

import "time"

type BookingStatus string

const (
	StatusPending   BookingStatus = "PENDING"
	StatusConfirmed BookingStatus = "CONFIRMED"
	StatusCancelled BookingStatus = "CANCELLED"
	StatusCompleted BookingStatus = "COMPLETED"
)

type Booking struct {
	ID            string        `db:"id"             json:"id"`
	WorkerID      string        `db:"worker_id"      json:"workerId"`
	WorkerName    string        `db:"worker_name"    json:"workerName"`
	WorkerAvatar  string        `db:"worker_avatar"  json:"workerAvatar"`
	CustomerID    string        `db:"customer_id"    json:"customerId"`
	CustomerName  string        `db:"customer_name"  json:"customerName"`
	Address       string        `db:"address"        json:"address"`
	City          string        `db:"city"           json:"city"`
	Latitude      float64       `db:"latitude"       json:"latitude"`
	Longitude     float64       `db:"longitude"      json:"longitude"`
	BookingDate   string        `db:"booking_date"   json:"bookingDate"`
	StartTime     string        `db:"start_time"     json:"startTime"`
	DurationDays  int           `db:"duration_days"  json:"durationDays"`
	PaymentMethod string        `db:"payment_method" json:"paymentMethod"`
	Status        BookingStatus `db:"status"         json:"status"`
	Notes         string        `db:"notes"          json:"notes"`
	JobID         string        `db:"job_id"         json:"jobId"`
	JobTitle      string        `db:"job_title"      json:"jobTitle"`
	CreatedAt     time.Time     `db:"created_at"     json:"createdAt"`
}

type CreateBookingRequest struct {
	WorkerID      string  `json:"workerId"       binding:"required"`
	WorkerName    string  `json:"workerName"`
	WorkerAvatar  string  `json:"workerAvatar"`
	CustomerName  string  `json:"customerName"   binding:"required"`
	Address       string  `json:"address"        binding:"required"`
	City          string  `json:"city"           binding:"required"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	BookingDate   string  `json:"bookingDate"    binding:"required"`
	StartTime     string  `json:"startTime"      binding:"required"`
	DurationDays  int     `json:"durationDays"   binding:"required,min=1"`
	PaymentMethod string  `json:"paymentMethod"  binding:"required"`
	Notes         string  `json:"notes"`
}
