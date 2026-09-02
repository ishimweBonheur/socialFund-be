package user

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CreateMemberRequest struct {
	FullName     string              `json:"full_name"`
	Email        string              `json:"email"`
	Phone        string              `json:"phone"`
	Contribution ContributionRequest `json:"contribution"`
	Reminder     ReminderRequest     `json:"reminder"`
}

type ContributionRequest struct {
	Amount            decimal.Decimal  `json:"amount"`
	Frequency         string           `json:"frequency"`
	IntervalValue     *int             `json:"interval_value,omitempty"`
	DueDay            *int             `json:"due_day,omitempty"`
	StartDate         string           `json:"start_date"`
	LateFeeEnabled    bool             `json:"late_fee_enabled"`
	LateFeePercentage *decimal.Decimal `json:"late_fee_percentage,omitempty"`
	GracePeriodDays   int              `json:"grace_period_days"`
}

type ReminderRequest struct {
	Enabled   bool   `json:"enabled"`
	Frequency string `json:"frequency"`
	Interval  *int   `json:"interval,omitempty"`
}

// UpdateInput is the HTTP and service input for a partial member update.
// Pointers distinguish an omitted field from a supplied empty value.
type UpdateInput struct {
	FullName *string `json:"full_name"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
}

// MemberResponse is the public representation returned after member creation.
// It deliberately omits internal fields such as GoogleID.
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
	return MemberResponse{
		ID:        u.ID,
		FullName:  u.FullName,
		Email:     u.Email,
		Phone:     u.Phone,
		Role:      u.Role,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
	}
}
