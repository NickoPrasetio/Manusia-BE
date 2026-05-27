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
	ID        string      `db:"id"         json:"id"`
	WorkerID  string      `db:"worker_id"  json:"workerId"`
	UserID    string      `db:"user_id"    json:"userId"`
	UserName  string      `db:"user_name"  json:"userName"`
	BookingID string      `db:"booking_id" json:"bookingId"`
	Rating    int         `db:"rating"     json:"rating"`
	Comment   string      `db:"comment"    json:"comment"`
	Photos    StringArray `db:"photos"     json:"photos"`
	Date      string      `db:"date"       json:"date"`
	CreatedAt time.Time   `db:"created_at" json:"createdAt"`
}

