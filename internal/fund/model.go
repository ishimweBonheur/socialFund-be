package fund

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

type FundTransaction struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	Type                string
	Direction           string
	Amount              decimal.Decimal
	ContributionID      *uuid.UUID
	AssistanceRequestID *uuid.UUID
	Reference           *string
	Description         *string
	RecordedBy          uuid.UUID
	CreatedAt           time.Time
}
