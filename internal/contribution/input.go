package contribution

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ApprovalInput struct {
	ContributionID uuid.UUID `json:"-"`
	AdminID        uuid.UUID `json:"admin_id"`
	Notes          *string   `json:"notes"`
}

type ProofInput struct {
	ContributionID       uuid.UUID
	UserID               uuid.UUID
	Amount               decimal.Decimal
	PaymentMethod        string
	TransactionReference string
	ProofURL             string
}

type Outstanding struct {
	OutstandingAmount decimal.Decimal `json:"outstanding_amount"`
	OverdueCount      int             `json:"overdue_count"`
}

type ReviewItem struct {
	Contribution
	MemberName  string          `json:"member_name"`
	MemberEmail string          `json:"member_email"`
	TotalDue    decimal.Decimal `json:"total_due"`
}
type ReviewTokenRequest struct {
	Token  string `json:"token"`
	Action string `json:"action"`
}
type ReviewPreview struct {
	Valid                               bool      `json:"valid"`
	Action                              string    `json:"action"`
	ContributionID                      uuid.UUID `json:"id"`
	MemberName                          string    `json:"member_name"`
	ExpectedAmount, LateFeeAmount       decimal.Decimal
	PaidAmount                          *decimal.Decimal
	PaymentMethod, TransactionReference *string
}
type RejectionInput struct {
	ContributionID uuid.UUID `json:"-"`
	AdminID        uuid.UUID `json:"admin_id"`
	Reason         string    `json:"reason"`
}
