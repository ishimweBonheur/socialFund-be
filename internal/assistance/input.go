package assistance

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type DisbursementInput struct {
	AssistanceRequestID uuid.UUID       `json:"-"`
	AdminID             uuid.UUID       `json:"admin_id"`
	Amount              decimal.Decimal `json:"amount"`
	Method              string          `json:"method"`
	Reference           string          `json:"reference"`
}
type CreateInput struct {
	UserID          uuid.UUID       `json:"-"`
	AmountRequested decimal.Decimal `json:"amount_requested"`
	Reason          string          `json:"reason"`
	Description     *string         `json:"description"`
	AttachmentURL   *string         `json:"attachment_url"`
}
type ApprovalInput struct {
	AssistanceRequestID uuid.UUID       `json:"-"`
	AdminID             uuid.UUID       `json:"-"`
	AmountApproved      decimal.Decimal `json:"amount_approved"`
}
type RejectionInput struct {
	AssistanceRequestID uuid.UUID `json:"-"`
	AdminID             uuid.UUID `json:"-"`
	Reason              string    `json:"reason"`
}
type ListFilter struct {
	UserID    *uuid.UUID
	Status    string
	Search    string
	DateFrom  string
	DateTo    string
	AmountMin string
	AmountMax string
	Limit     int
	Offset    int
}
type ReviewItem struct {
	AssistanceRequest
	MemberName  string `json:"member_name"`
	MemberEmail string `json:"member_email"`
}
