package notification

import (
	"context"
	"fmt"

	mail "github.com/wneessen/go-mail"
)

type EmailSender interface {
	SendAccountCreated(context.Context, AccountCreatedEmailData) error
}
type mailClient interface {
	DialAndSendWithContext(context.Context, ...*mail.Msg) error
}
type GoMailSender struct {
	client mailClient
	from   string
}

func NewGoMailSender(host string, port int, username, password, from string) (*GoMailSender, error) {
	if host == "" || port < 1 || port > 65535 || username == "" || password == "" || from == "" {
		return nil, fmt.Errorf("SMTP host, port, username, password, and from address are required")
	}
	client, err := mail.NewClient(host, mail.WithPort(port), mail.WithSMTPAuth(mail.SMTPAuthPlain), mail.WithUsername(username), mail.WithPassword(password), mail.WithTLSPolicy(mail.TLSMandatory))
	if err != nil {
		return nil, fmt.Errorf("create SMTP client: %w", err)
	}
	return &GoMailSender{client: client, from: from}, nil
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
	if err = message.From(s.from); err != nil {
		return fmt.Errorf("set welcome email sender: %w", err)
	}
	if err = message.To(recipient); err != nil {
		return fmt.Errorf("set welcome email recipient: %w", err)
	}
	message.Subject(accountCreatedSubject)
	message.SetBodyString(mail.TypeTextPlain, plainBody)
	message.AddAlternativeString(mail.TypeTextHTML, htmlBody)
	if err = s.client.DialAndSendWithContext(ctx, message); err != nil {
		return fmt.Errorf("send account created email: %w", err)
	}
	return nil
}
