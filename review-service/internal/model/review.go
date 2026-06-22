package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StringArray stored as JSONB
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	b, _ := json.Marshal(s)
	return string(b), nil
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
	return fmt.Errorf("cannot scan %T", src)
}

type Review struct {
	ID           string      `db:"id"            json:"id"`
	WorkerID     string      `db:"worker_id"     json:"workerId"`
	WorkerName   string      `db:"worker_name"   json:"workerName"`
	WorkerAvatar string      `db:"worker_avatar" json:"workerAvatar"`
	UserID       string      `db:"user_id"       json:"userId"`
	UserName     string      `db:"user_name"     json:"userName"`
	BookingID    string      `db:"booking_id"    json:"bookingId"`
	Rating       int         `db:"rating"        json:"rating"`
	Comment      string      `db:"comment"       json:"comment"`
	Photos       StringArray `db:"photos"        json:"photos"`
	Date         string      `db:"date"          json:"date"`
	EditCount    int         `db:"edit_count"    json:"editCount"`
	CreatedAt    time.Time   `db:"created_at"    json:"createdAt"`
}

// RatingDist is a single star-level count for the distribution chart.
type RatingDist struct {
	Star  int `json:"star"`
	Count int `json:"count"`
}

// ReviewPage is the paginated response for the worker's review list.
type ReviewPage struct {
	Reviews   []Review     `json:"reviews"`
	Total     int          `json:"total"`
	AvgRating float64      `json:"avgRating"`
	Dist      []RatingDist `json:"dist"`
	Page      int          `json:"page"`
	Limit     int          `json:"limit"`
	Last      bool         `json:"last"`
}

// GivenReviewPage is the paginated response for reviews a user has given.
type GivenReviewPage struct {
	Reviews []Review `json:"reviews"`
	Total   int      `json:"total"`
	Page    int      `json:"page"`
	Limit   int      `json:"limit"`
	Last    bool     `json:"last"`
}

