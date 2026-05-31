package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type WorkStatus string

const (
	WorkStatusOpen   WorkStatus = "OPEN"
	WorkStatusClosed WorkStatus = "CLOSED"
	WorkStatusBooked WorkStatus = "BOOKED"
)

// StringArray is a custom type for PostgreSQL text[]
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *StringArray) Scan(src interface{}) error {
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	case nil:
		*s = StringArray{}
		return nil
	}
	return fmt.Errorf("cannot scan type %T into StringArray", src)
}

// UserProfile is the worker profile entity
type UserProfile struct {
	ID              string      `db:"id"               json:"id"`
	AuthID          string      `db:"auth_id"          json:"authId"`
	Name            string      `db:"name"             json:"name"`
	Avatar          string      `db:"avatar"           json:"avatar"`
	Age             int         `db:"age"              json:"age"`
	Experience      int         `db:"experience"       json:"experience"`
	Rating          float64     `db:"rating"           json:"rating"`
	TotalReviews    int         `db:"total_reviews"    json:"totalReviews"`
	Specializations StringArray `db:"specializations"  json:"specializations"`
	Location        string      `db:"location"         json:"location"`
	PricePerDay     int64       `db:"price_per_day"    json:"pricePerDay"`
	IsAvailable     bool        `db:"is_available"     json:"isAvailable"`
	WorkStatus      WorkStatus  `db:"work_status"      json:"workStatus"`
	Bio             string      `db:"bio"              json:"bio"`
	Gender          string      `db:"gender"           json:"gender"`
	BirthPlace      string      `db:"birth_place"      json:"birthPlace"`
	Latitude        *float64    `db:"latitude"         json:"latitude,omitempty"`
	Longitude       *float64    `db:"longitude"        json:"longitude,omitempty"`
	CreatedAt       time.Time   `db:"created_at"       json:"createdAt"`
}

// CreateProfileRequest for POST /api/users
type CreateProfileRequest struct {
	AuthID          string     `json:"authId"          binding:"required"`
	Name            string     `json:"name"            binding:"required"`
	Age             int        `json:"age"`
	Experience      int        `json:"experience"`
	Specializations []string   `json:"specializations"`
	Location        string     `json:"location"`
	PricePerDay     int64      `json:"pricePerDay"`
	Bio             string     `json:"bio"`
}

// UpdateProfileRequest for PUT /api/users/:id or PUT /api/users/me/profile
type UpdateProfileRequest struct {
	Name            string   `json:"name"`
	Age             int      `json:"age"`
	Experience      int      `json:"experience"`
	Specializations []string `json:"specializations"`
	Location        string   `json:"location"`
	PricePerDay     int64    `json:"pricePerDay"`
	Bio             string   `json:"bio"`
	IsAvailable     *bool    `json:"isAvailable"`
	WorkStatus      string   `json:"workStatus"`
	Gender          string   `json:"gender"`
	BirthPlace      string   `json:"birthPlace"`
}

// ProfilePage is the paginated response
type ProfilePage struct {
	Content       []UserProfile `json:"content"`
	TotalElements int           `json:"totalElements"`
	TotalPages    int           `json:"totalPages"`
	Number        int           `json:"number"`
	Last          bool          `json:"last"`
}
