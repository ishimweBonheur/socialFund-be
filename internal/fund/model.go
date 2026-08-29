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
	Status              string          `json:"status"`
	PaymentMethod       *string         `json:"payment_method,omitempty"`
	ContributionID      *uuid.UUID      `json:"contribution_id,omitempty"`
	AssistanceRequestID *uuid.UUID      `json:"assistance_request_id,omitempty"`
	Reference           *string         `json:"reference,omitempty"`
	Description         *string         `json:"description,omitempty"`
	RecordedBy          uuid.UUID       `json:"recorded_by"`
	CreatedAt           time.Time       `json:"created_at"`
}
type MemberSummary struct {
	TotalContributed   decimal.Decimal `json:"total_contributed"`
	AssistanceReceived decimal.Decimal `json:"assistance_received"`
	NetPosition        decimal.Decimal `json:"net_position"`
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
