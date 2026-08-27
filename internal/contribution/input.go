package contribution

import "github.com/google/uuid"

type ApprovalInput struct {
	ContributionID uuid.UUID `json:"-"`
	AdminID        uuid.UUID `json:"admin_id"`
	Notes          *string   `json:"notes"`
}
type RejectionInput struct {
	ContributionID uuid.UUID `json:"-"`
	AdminID        uuid.UUID `json:"admin_id"`
	Reason         string    `json:"reason"`
}
