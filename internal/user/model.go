package user

import (
	"github.com/google/uuid"
	"time"
)

type User struct {
	ID          uuid.UUID  `json:"id"`
	FullName    string     `json:"full_name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	GoogleID    *string    `json:"-"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
type ListFilter struct {
	Status, Role, Search string
	DateFrom, DateTo     string
	Limit, Offset        int
}
