package notification

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	mail "github.com/wneessen/go-mail"
)

type EmailSender interface {
	SendAccountCreated(context.Context, AccountCreatedEmailData) error
	SendNotification(context.Context, Notification) error
}
type AttachmentLoader interface {
	Open(context.Context, string) (io.ReadCloser, string, error)
}
type mailClient interface {
	DialAndSendWithContext(context.Context, ...*mail.Msg) error
}
type GoMailSender struct {
	client      mailClient
	from        string
	attachments AttachmentLoader
	logo        []byte
}

func NewGoMailSender(host string, port int, username, password, from string, loaders ...AttachmentLoader) (*GoMailSender, error) {
	if host == "" || port < 1 || port > 65535 || username == "" || password == "" || from == "" {
		return nil, fmt.Errorf("SMTP host, port, username, password, and from address are required")
	}
	client, err := mail.NewClient(host, mail.WithPort(port), mail.WithSMTPAuth(mail.SMTPAuthPlain), mail.WithUsername(username), mail.WithPassword(password), mail.WithTLSPolicy(mail.TLSMandatory))
	if err != nil {
		return nil, fmt.Errorf("create SMTP client: %w", err)
	}
	var loader AttachmentLoader
	if len(loaders) > 0 {
		loader = loaders[0]
	}
	logo, _ := os.ReadFile("social-fund-icon.png")
	return &GoMailSender{client: client, from: from, attachments: loader, logo: logo}, nil
}
func (s *GoMailSender) SendAccountCreated(ctx context.Context, data AccountCreatedEmailData) error {
	htmlBody, plainBody, err := renderAccountCreated(data)
	if err != nil {
		return err
	}
	recipient := data.Recipient
	if recipient == "" {
		recipient = data.Email
	}
	message := mail.NewMsg()
	if err = message.FromFormat("Social Fund", s.from); err != nil {
		return fmt.Errorf("set welcome email sender: %w", err)
	}
	if err = message.To(recipient); err != nil {
		return fmt.Errorf("set welcome email recipient: %w", err)
	}
	message.Subject(accountCreatedSubject)
	message.SetBodyString(mail.TypeTextPlain, plainBody)
	message.AddAlternativeString(mail.TypeTextHTML, htmlBody)
	if len(s.logo) > 0 {
		if err = message.EmbedReader("social-fund-icon.png", bytes.NewReader(s.logo), mail.WithFileContentID(logoContentID)); err != nil {
			return fmt.Errorf("embed Social Fund logo: %w", err)
		}
	}
	if err = s.client.DialAndSendWithContext(ctx, message); err != nil {
		return fmt.Errorf("send account created email: %w", err)
	}
	return nil
}

func (s *GoMailSender) SendNotification(ctx context.Context, notification Notification) error {
	if notification.Subject == nil || notification.Message == nil {
		return fmt.Errorf("notification content is missing")
	}
	htmlBody, plainBody, err := renderNotification(notification)
	if err != nil {
		return err
	}
	message := mail.NewMsg()
	if err = message.FromFormat("Social Fund", s.from); err != nil {
		return fmt.Errorf("set notification sender: %w", err)
	}
	if err = message.To(notification.Recipient); err != nil {
		return fmt.Errorf("set notification recipient: %w", err)
	}
	message.Subject(*notification.Subject)
	message.SetBodyString(mail.TypeTextPlain, plainBody)
	message.AddAlternativeString(mail.TypeTextHTML, htmlBody)
	if len(s.logo) > 0 {
		if err = message.EmbedReader("social-fund-icon.png", bytes.NewReader(s.logo), mail.WithFileContentID(logoContentID)); err != nil {
			return fmt.Errorf("embed Social Fund logo: %w", err)
		}
	}
	if notification.Reminder != nil && len(notification.Reminder.QRCodeBytes) > 0 {
		if err = message.EmbedReader("payment-qr.png", bytes.NewReader(notification.Reminder.QRCodeBytes), mail.WithFileContentID("social-fund-payment-qr")); err != nil {
			return fmt.Errorf("embed payment QR code: %w", err)
		}
	}
	if notification.AttachmentKey != nil && s.attachments != nil {
		reader, filename, openErr := s.attachments.Open(ctx, *notification.AttachmentKey)
		if openErr != nil {
			return fmt.Errorf("open notification attachment: %w", openErr)
		}
		defer reader.Close()
		if err = message.AttachReader(filename, reader); err != nil {
			return fmt.Errorf("attach payment proof: %w", err)
		}
	}
	if err = s.client.DialAndSendWithContext(ctx, message); err != nil {
		return fmt.Errorf("send notification email: %w", err)
	}
	return nil
}
