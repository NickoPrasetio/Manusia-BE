package model

import "time"

type JobCategory string
type JobStatus string

const (
	CategoryTask    JobCategory = "TASK"
	CategoryProject JobCategory = "PROJECT"
	CategoryEvent   JobCategory = "EVENT"

	JobStatusOpen   JobStatus = "OPEN"
	JobStatusClosed JobStatus = "CLOSED"
)

// Job represents a work posting created by a customer.
// Workers can browse open jobs and apply (future feature).
type Job struct {
	ID           string      `db:"id"             json:"id"`
	CustomerID   string      `db:"customer_id"    json:"customerId"`
	CustomerName string      `db:"customer_name"  json:"customerName"`
	Title        string      `db:"title"          json:"title"`
	Description  string      `db:"description"    json:"description"`
	BudgetPerDay int64       `db:"budget_per_day" json:"budgetPerDay"`
	TodoList     []string    `db:"-"              json:"todoList"`
	DurationDays int         `db:"duration_days"  json:"durationDays"`
	City         string      `db:"city"           json:"city"`
	Latitude     float64     `db:"latitude"       json:"latitude"`
	Longitude    float64     `db:"longitude"      json:"longitude"`
	Category     JobCategory `db:"category"       json:"category"`
	Status       JobStatus   `db:"status"         json:"status"`
	CreatedAt    time.Time   `db:"created_at"     json:"createdAt"`
}

type CreateJobRequest struct {
	CustomerName string      `json:"customerName" binding:"required"`
	Title        string      `json:"title"        binding:"required"`
	Description  string      `json:"description"  binding:"required"`
	BudgetPerDay int64       `json:"budgetPerDay" binding:"required,min=1"`
	TodoList     []string    `json:"todoList"`
	DurationDays int         `json:"durationDays" binding:"required,min=1"`
	City         string      `json:"city"         binding:"required"`
	Latitude     float64     `json:"latitude"`
	Longitude    float64     `json:"longitude"`
	Category     JobCategory `json:"category"     binding:"required"`
}
