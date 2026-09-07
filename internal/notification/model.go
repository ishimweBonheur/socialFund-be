package notification

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	ContributionID *uuid.UUID
	Type           string
	Channel        string
	Recipient      string
	Subject        *string
	Message        *string
	Status         string
	Attempts       int
	LastError      *string
	NextRetryAt    *time.Time
	SentAt         *time.Time
	ReadAt         *time.Time
	AttachmentKey  *string
	ProofURL       *string
	ApproveURL     *string
	RejectURL      *string
	CreatedAt      time.Time
	LogoURL        string
	PaymentURL     string               `json:"-"`
	Reminder       *PaymentReminderData `json:"-"`
}

type PaymentReminderData struct {
	MemberName         string
	AmountDue          string
	OriginalAmount     string
	LateFee            string
	TotalAmountDue     string
	DueDate            string
	DaysUntilDue       int
	DaysOverdue        int
	AccountName        string
	PaymentType        string
	PaymentMethodLabel string
	PhoneNumber        string
	MerchantCode       string
	USSDCode           string
	QRCodeData         string
	QRCodeBytes        []byte `json:"-"`
}
type Filter struct {
	Status, Type, DateFrom, DateTo string
	UserID                         *uuid.UUID
	Limit, Offset                  int
}
