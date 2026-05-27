package model

import "time"

// UserType represents whether the user is a customer or a worker
type UserType string

const (
	UserTypeCustomer UserType = "CUSTOMER"
	UserTypeWorker   UserType = "WORKER"
)

// User is the core auth entity stored in manusia_auth database
type User struct {
	ID           string    `db:"id"            json:"id"`
	Name         string    `db:"name"          json:"name"`
	Email        string    `db:"email"         json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Phone        string    `db:"phone"         json:"phone"`
	Role         string    `db:"role"          json:"role"`
	UserType     UserType  `db:"user_type"     json:"userType"`
	Avatar       string    `db:"avatar"        json:"avatar"`
	NIK          string    `db:"nik"           json:"nik"`
	BirthDate    string    `db:"birth_date"    json:"birthDate"`
	Gender       string    `db:"gender"        json:"gender"`
	KTPPhoto     string    `db:"ktp_photo"     json:"ktpPhoto"`
	Latitude     *float64  `db:"latitude"      json:"latitude,omitempty"`
	Longitude    *float64  `db:"longitude"     json:"longitude,omitempty"`
	CreatedAt    time.Time `db:"created_at"    json:"createdAt"`
}

// RegisterRequest is the payload for POST /api/auth/register (multipart/form-data)
type RegisterRequest struct {
	Name      string   `form:"name"`
	Email     string   `form:"email"`
	Password  string   `form:"password"`
	Phone     string   `form:"phone"`
	UserType  UserType `form:"userType"`
	NIK       string   `form:"nik"`
	BirthDate string   `form:"birthDate"`
	Gender    string   `form:"gender"`
	Latitude  *float64 `form:"latitude"`
	Longitude *float64 `form:"longitude"`
	KTPPhoto  string   // set by handler after upload
}

// LoginRequest is the payload for POST /api/auth/login
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse is returned after successful auth
type AuthResponse struct {
	Token     string   `json:"token"`
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	Phone     string   `json:"phone"`
	Role      string   `json:"role"`
	UserType  UserType `json:"userType"`
	Avatar    string   `json:"avatar"`
	NIK       string   `json:"nik"`
	BirthDate string   `json:"birthDate"`
	Gender    string   `json:"gender"`
	KTPPhoto  string   `json:"ktpPhoto"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// UpdateProfileRequest for PUT /api/auth/me
type UpdateProfileRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}
