package contributionplan

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

type ContributionPlan struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Amount            decimal.Decimal
	Frequency         string
	IntervalValue     *int
	DueDay            *int
	StartDate         time.Time
	EndDate           *time.Time
	ReminderEnabled   bool
	ReminderFrequency *string
	ReminderInterval  *int
	LateFeeEnabled    bool
	LateFeePercentage *decimal.Decimal
	GracePeriodDays   int
	IsActive          bool
	CreatedBy         uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ListItem struct {
	ContributionPlan
	MemberName  string `json:"member_name"`
	MemberEmail string `json:"member_email"`
}
