package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type AccountCreatedDataLoader interface {
	LoadAccountCreatedEmailData(context.Context, uuid.UUID) (AccountCreatedEmailData, error)
}
type RoutingSender struct {
	fallback    Sender
	loader      AccountCreatedDataLoader
	email       EmailSender
	frontendURL string
	logger      *slog.Logger
}

func NewRoutingSender(fallback Sender, loader AccountCreatedDataLoader, email EmailSender, frontendURL string, logger *slog.Logger) *RoutingSender {
	return &RoutingSender{fallback: fallback, loader: loader, email: email, frontendURL: frontendURL, logger: logger}
}
func (s *RoutingSender) Send(ctx context.Context, n Notification) error {
	started := time.Now()
	var err error
	if n.Type == "ACCOUNT_CREATED" {
		var data AccountCreatedEmailData
		data, err = s.loader.LoadAccountCreatedEmailData(ctx, n.UserID)
		if err == nil {
			data.LoginURL = BuildLoginURL(s.frontendURL)
			data.Recipient = n.Recipient
			err = s.email.SendAccountCreated(ctx, data)
		}
	} else {
		err = s.email.SendNotification(ctx, n)
	}
	status := "sent"
	if err != nil {
		status = "failed"
	}
	s.logger.InfoContext(ctx, "account created email delivery", "notification_id", n.ID, "notification_type", n.Type, "recipient", n.Recipient, "status", status, "duration_ms", time.Since(started).Milliseconds())
	if err != nil {
		return fmt.Errorf("deliver %s notification: %w", n.Type, err)
	}
	return nil
}
