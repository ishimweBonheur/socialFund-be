package user

import (
	"github.com/google/uuid"
	"time"
)

type User struct {
	ID          uuid.UUID
	FullName    string
	Email       string
	Phone       string
	GoogleID    *string
	Role        string
	Status      string
	LastLoginAt *time.Time
	CreatedBy   *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
