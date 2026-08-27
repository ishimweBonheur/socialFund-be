package user

import "github.com/shopspring/decimal"

type CreateMemberRequest struct {
	FullName     string              `json:"full_name"`
	Email        string              `json:"email"`
	Phone        string              `json:"phone"`
	Contribution ContributionRequest `json:"contribution"`
	Reminder     ReminderRequest     `json:"reminder"`
}
type ContributionRequest struct {
	Amount        decimal.Decimal `json:"amount"`
	Frequency     string          `json:"frequency"`
	IntervalValue *int            `json:"interval_value,omitempty"`
	DueDay        *int            `json:"due_day,omitempty"`
	StartDate     string          `json:"start_date"`
}
type ReminderRequest struct {
	Enabled   bool   `json:"enabled"`
	Frequency string `json:"frequency"`
	Interval  *int   `json:"interval,omitempty"`
}
