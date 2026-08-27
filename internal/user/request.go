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
