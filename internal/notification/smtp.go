package notification

import (
	"context"
	"fmt"
	"net/smtp"
)

type SMTPSender struct{ host, port, username, password, from string }

func NewSMTPSender(host, port, username, password, from string) *SMTPSender {
	return &SMTPSender{host: host, port: port, username: username, password: password, from: from}
}
func (s *SMTPSender) Send(ctx context.Context, n Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n.Subject == nil || n.Message == nil {
		return fmt.Errorf("notification content is missing")
	}
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}
	message := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", s.from, n.Recipient, *n.Subject, *n.Message))
	if err := smtp.SendMail(s.host+":"+s.port, auth, s.from, []string{n.Recipient}, message); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
