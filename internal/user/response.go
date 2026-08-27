package user

import (
	"github.com/google/uuid"
	"time"
)

type MemberResponse struct {
	ID        uuid.UUID `json:"id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func responseFromUser(u User) MemberResponse {
	return MemberResponse{ID: u.ID, FullName: u.FullName, Email: u.Email, Phone: u.Phone, Role: u.Role, Status: u.Status, CreatedAt: u.CreatedAt}
}
