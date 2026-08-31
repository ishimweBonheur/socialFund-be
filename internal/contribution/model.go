package contribution

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

type Contribution struct {
	ID                     uuid.UUID
	UserID                 uuid.UUID
	ContributionPlanID     uuid.UUID
	ExpectedAmount         decimal.Decimal
	LateFeePercentage      *decimal.Decimal
	LateFeeAmount          decimal.Decimal
	OverdueAt              *time.Time
	DueDate                time.Time
	PaidAmount             *decimal.Decimal
	PaymentDate            *time.Time
	PaymentMethod          *string
	TransactionReference   *string
	ProofURL               *string
	ProofUploadedAt        *time.Time
	Status                 string
	RejectionReason        *string
	ApprovedBy             *uuid.UUID
	ApprovedAt             *time.Time
	ApprovalTokenHash      *string
	ApprovalTokenExpiresAt *time.Time
	ApprovalTokenUsedAt    *time.Time
	ApprovalTokenAction    *string
	Notes                  *string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
type AdminListFilter struct {
	Search, Status, DueFrom, DueTo, Method, Proof, PaymentState string
	LateFee, Reference, PaidFrom, PaidTo, AmountMin, AmountMax  string
	Limit, Offset                                               int
}

func (c Contribution) TotalDue() decimal.Decimal {
	return c.ExpectedAmount.Add(c.LateFeeAmount)
}
