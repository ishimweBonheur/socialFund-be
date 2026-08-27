package fund

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

type FundTransaction struct {
	ID                  uuid.UUID       `json:"id"`
	UserID              uuid.UUID       `json:"user_id"`
	UserName            string          `json:"user_name,omitempty"`
	Type                string          `json:"type"`
	Direction           string          `json:"direction"`
	Amount              decimal.Decimal `json:"amount"`
	ContributionID      *uuid.UUID
	AssistanceRequestID *uuid.UUID
	Reference           *string
	Description         *string
	RecordedBy          uuid.UUID
	CreatedAt           time.Time
}
type Summary struct {
	TotalIn  decimal.Decimal `json:"total_in"`
	TotalOut decimal.Decimal `json:"total_out"`
	Balance  decimal.Decimal `json:"balance"`
}
type Filter struct {
	Type, Direction, DateFrom, DateTo string
	UserID                            *uuid.UUID
	Limit, Offset                     int
}
