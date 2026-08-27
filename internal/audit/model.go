package audit

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type AuditLog struct {
	ID         uuid.UUID
	UserID     *uuid.UUID
	Action     string
	EntityType string
	EntityID   uuid.UUID
	OldData    json.RawMessage
	NewData    json.RawMessage
	IPAddress  *string
	UserAgent  *string
	CreatedAt  time.Time
}
