package notification

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AccountCreatedDataLoader interface {
	LoadAccountCreatedEmailData(context.Context, uuid.UUID) (AccountCreatedEmailData, error)
	LoadPaymentReminderData(context.Context, *uuid.UUID) (*PaymentReminderData, error)
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
			data.LogoURL = "cid:" + logoContentID
			data.Recipient = n.Recipient
			err = s.email.SendAccountCreated(ctx, data)
		}
	} else {
		n.LogoURL = "cid:" + logoContentID
		if n.Type == "CONTRIBUTION_DUE" || n.Type == "CONTRIBUTION_OVERDUE" {
			n.PaymentURL = strings.TrimRight(s.frontendURL, "/") + "/dashboard"
			if err == nil {
				n.Reminder, err = s.loader.LoadPaymentReminderData(ctx, n.ContributionID)
			}
		}
		if err == nil {
			err = s.email.SendNotification(ctx, n)
		}
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
