package notification

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	texttemplate "text/template"

	"github.com/shopspring/decimal"
)

const accountCreatedSubject = "Your Social Fund Account Has Been Created"

type AccountCreatedEmailData struct {
	FullName              string
	Email                 string
	Phone                 string
	ContributionAmount    string
	ContributionFrequency string
	PaymentDue            string
	LoginURL              string
	Recipient             string
}

func BuildLoginURL(frontendURL string) string { return strings.TrimRight(frontendURL, "/") + "/login" }

var htmlTemplate = template.Must(template.New("account-created").Parse(`<!doctype html><html><body style="margin:0;background:#f4f6f8;font-family:Arial,sans-serif;color:#17212b"><table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td style="padding:24px 12px"><table role="presentation" width="100%" style="max-width:600px;margin:auto;background:#fff;border:1px solid #dde3e8"><tr><td style="padding:28px;background:#173f5f;color:#fff;font-size:22px;font-weight:bold">Social Fund</td></tr><tr><td style="padding:28px"><p style="font-size:17px">Hello {{.FullName}},</p><p>Your Social Fund account has been created successfully.</p><h2 style="font-size:17px;margin-top:28px">Account Details</h2><table role="presentation" width="100%" cellspacing="0" cellpadding="8" style="background:#f7f9fa;border:1px solid #e3e8ec"><tr><td><strong>Name</strong></td><td>{{.FullName}}</td></tr><tr><td><strong>Email</strong></td><td>{{.Email}}</td></tr><tr><td><strong>Phone</strong></td><td>{{.Phone}}</td></tr><tr><td><strong>Contribution Amount</strong></td><td>{{.ContributionAmount}}</td></tr><tr><td><strong>Contribution Frequency</strong></td><td>{{.ContributionFrequency}}</td></tr><tr><td><strong>Payment Due</strong></td><td>{{.PaymentDue}}</td></tr></table><p style="margin-top:24px"><strong>Your account is currently inactive.</strong></p><p>To access your account, sign in with Google using:</p><p style="font-weight:bold">{{.Email}}</p><p style="margin:28px 0"><a href="{{.LoginURL}}" style="display:inline-block;background:#167d68;color:#fff;text-decoration:none;padding:13px 22px;border-radius:4px;font-weight:bold">Login to Social Fund</a></p><p style="font-size:14px;color:#52616b">Your account will become active after your registered Google account is successfully verified.</p></td></tr></table></td></tr></table></body></html>`))

var plainTemplate = texttemplate.Must(texttemplate.New("account-created").Parse(`Hello {{.FullName}},

Your Social Fund account has been created successfully.

Name: {{.FullName}}
Email: {{.Email}}
Phone: {{.Phone}}
Contribution Amount: {{.ContributionAmount}}
Contribution Frequency: {{.ContributionFrequency}}
Payment Due: {{.PaymentDue}}

Your account is currently inactive.

Login using Google with:
{{.Email}}

Login:
{{.LoginURL}}

Your account will become active after your registered Google account is successfully verified.`))

func renderAccountCreated(data AccountCreatedEmailData) (string, string, error) {
	var htmlBody, plainBody bytes.Buffer
	if err := htmlTemplate.Execute(&htmlBody, data); err != nil {
		return "", "", fmt.Errorf("render HTML welcome email: %w", err)
	}
	if err := plainTemplate.Execute(&plainBody, data); err != nil {
		return "", "", fmt.Errorf("render plain welcome email: %w", err)
	}
	return htmlBody.String(), plainBody.String(), nil
}
func formatAmount(value decimal.Decimal) string {
	parts := strings.Split(value.StringFixed(2), ".")
	whole := parts[0]
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	return whole + "." + parts[1] + " RWF"
}
func formatFrequency(value string) string {
	value = strings.ToLower(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
func formatPaymentDue(frequency string, dueDay, interval *int) string {
	switch frequency {
	case "MONTHLY":
		if dueDay != nil {
			return fmt.Sprintf("%s of every month", ordinal(*dueDay))
		}
	case "DAILY":
		return "Every day"
	case "WEEKLY":
		return "Every week"
	case "CUSTOM":
		if interval != nil {
			return fmt.Sprintf("Every %d days", *interval)
		}
	}
	return "As scheduled"
}
func ordinal(value int) string {
	suffix := "th"
	if value%100 < 11 || value%100 > 13 {
		switch value % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", value, suffix)
}
