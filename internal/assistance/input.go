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
