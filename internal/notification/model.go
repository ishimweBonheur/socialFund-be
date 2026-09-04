package notification

import (
	"github.com/google/uuid"
	"time"
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
}
type Filter struct {
	Status, Type, DateFrom, DateTo string
	UserID                         *uuid.UUID
	Limit, Offset                  int
}
