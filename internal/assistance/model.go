package assistance

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

type AssistanceRequest struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	AmountRequested       decimal.Decimal
	Reason                string
	Description           *string
	AttachmentURL         *string
	Status                string
	AmountApproved        *decimal.Decimal
	ReviewedBy            *uuid.UUID
	ReviewedAt            *time.Time
	RejectionReason       *string
	AmountDisbursed       *decimal.Decimal
	DisbursementMethod    *string
	DisbursementReference *string
	DisbursedBy           *uuid.UUID
	DisbursedAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
