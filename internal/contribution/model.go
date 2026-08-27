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
	Notes                  *string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
