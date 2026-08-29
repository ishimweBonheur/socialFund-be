package notification

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"regexp"
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

func BuildLoginURL(frontendURL string) string {
	return strings.TrimRight(frontendURL, "/") + "/login"
}

var htmlTemplate = template.Must(template.New("account-created").Parse(`<!doctype html><html><body style="margin:0;background:#f5f7f8;font-family:Arial,sans-serif;color:#111827"><table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td style="padding:28px 12px"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:600px;margin:auto;background:#fff;border:1px solid #e5e7eb;border-radius:16px;overflow:hidden"><tr><td style="padding:24px 28px;background:#0f9d58;color:#fff;font-size:22px;font-weight:bold">Social Fund</td></tr><tr><td style="padding:30px 28px"><div style="display:inline-block;padding:6px 12px;border-radius:999px;background:#ecfdf5;color:#0f9d58;font-size:12px;font-weight:bold">ACCOUNT CREATED</div><h1 style="font-size:22px;margin:18px 0 8px">Welcome, {{.FullName}}</h1><p style="color:#4b5563;line-height:1.6">Your Social Fund account has been created successfully.</p><h2 style="font-size:16px;margin-top:28px">Account details</h2><table role="presentation" width="100%" cellspacing="0" cellpadding="10" style="background:#f5f7f8;border:1px solid #e5e7eb;border-radius:10px"><tr><td><strong>Name</strong></td><td>{{.FullName}}</td></tr><tr><td><strong>Email</strong></td><td>{{.Email}}</td></tr><tr><td><strong>Phone</strong></td><td>{{.Phone}}</td></tr><tr><td><strong>Contribution amount</strong></td><td>{{.ContributionAmount}}</td></tr><tr><td><strong>Contribution frequency</strong></td><td>{{.ContributionFrequency}}</td></tr><tr><td><strong>Payment due</strong></td><td>{{.PaymentDue}}</td></tr></table><p style="margin-top:24px"><strong>Your account is currently inactive.</strong></p><p style="color:#4b5563;line-height:1.6">Sign in with your registered Google account ({{.Email}}) to verify and activate it.</p><p style="margin:28px 0"><a href="{{.LoginURL}}" style="display:inline-block;background:#0f9d58;color:#fff;text-decoration:none;padding:13px 22px;border-radius:999px;font-weight:bold">Login to Social Fund</a></p></td></tr><tr><td style="padding:18px 28px;background:#f5f7f8;color:#6b7280;font-size:12px">This is an automated message from Social Fund.</td></tr></table></td></tr></table></body></html>`))

type notificationEmailData struct {
	Subject, Label, Accent, Tint    string
	Body                            template.HTML
	ProofURL, ApproveURL, RejectURL string
}

var urlPattern = regexp.MustCompile(`https?://[^\s<]+`)
var notificationHTMLTemplate = template.Must(template.New("notification").Parse(`<!doctype html><html><body style="margin:0;background:#f5f7f8;font-family:Arial,sans-serif;color:#111827"><table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td style="padding:28px 12px"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:600px;margin:auto;background:#fff;border:1px solid #e5e7eb;border-radius:16px;overflow:hidden"><tr><td style="padding:24px 28px;background:#0f9d58;color:#fff;font-size:22px;font-weight:bold">Social Fund</td></tr><tr><td style="padding:30px 28px"><div style="display:inline-block;padding:6px 12px;border-radius:999px;background:{{.Tint}};color:{{.Accent}};font-size:12px;font-weight:bold">{{.Label}}</div><h1 style="font-size:22px;line-height:1.3;margin:18px 0">{{.Subject}}</h1><div style="border-left:4px solid {{.Accent}};background:#f9fafb;border-radius:8px;padding:18px;color:#374151;font-size:15px;line-height:1.7">{{.Body}}</div>{{if .ProofURL}}<p style="margin:24px 0 10px"><a href="{{.ProofURL}}" style="display:inline-block;background:#374151;color:#fff;text-decoration:none;padding:12px 18px;border-radius:999px;font-weight:bold">View Proof</a></p>{{end}}{{if .ApproveURL}}<table role="presentation" cellspacing="0" cellpadding="0" style="margin-top:12px"><tr><td style="padding-right:10px"><a href="{{.ApproveURL}}" style="display:inline-block;background:#0f9d58;color:#fff;text-decoration:none;padding:12px 20px;border-radius:999px;font-weight:bold">Approve</a></td><td><a href="{{.RejectURL}}" style="display:inline-block;background:#dc2626;color:#fff;text-decoration:none;padding:12px 20px;border-radius:999px;font-weight:bold">Reject</a></td></tr></table>{{end}}{{if .ProofURL}}<p style="margin-top:18px;color:#6b7280;font-size:13px">The original payment proof is also attached to this email.</p>{{end}}</td></tr><tr><td style="padding:18px 28px;background:#f5f7f8;color:#6b7280;font-size:12px">This is an automated message from Social Fund. Please keep transaction references for your records.</td></tr></table></td></tr></table></body></html>`))

func notificationAppearance(kind string) (string, string, string) {
	switch kind {
	case "CONTRIBUTION_APPROVED", "ASSISTANCE_APPROVED", "ASSISTANCE_PAID":
		return "SUCCESS", "#0f9d58", "#ecfdf5"
	case "CONTRIBUTION_REJECTED", "ASSISTANCE_REJECTED":
		return "ACTION REQUIRED", "#dc2626", "#fef2f2"
	case "CONTRIBUTION_OVERDUE":
		return "PAYMENT OVERDUE", "#d97706", "#fffbeb"
	case "PROOF_SUBMITTED":
		return "REVIEW REQUIRED", "#0f9d58", "#ecfdf5"
	default:
		return "NOTIFICATION", "#0f9d58", "#ecfdf5"
	}
}

func renderNotification(n Notification) (string, string, error) {
	if n.Subject == nil || n.Message == nil {
		return "", "", fmt.Errorf("notification content is missing")
	}
	label, accent, tint := notificationAppearance(n.Type)
	escaped := html.EscapeString(*n.Message)
	escaped = urlPattern.ReplaceAllStringFunc(escaped, func(value string) string {
		return `<a href="` + value + `" style="color:#0f9d58;font-weight:bold;word-break:break-all">` + value + `</a>`
	})
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")
	data := notificationEmailData{Subject: *n.Subject, Label: label, Accent: accent, Tint: tint, Body: template.HTML(escaped)} // #nosec G203 -- content is escaped before URLs and line breaks are added.
	if n.ProofURL != nil {
		data.ProofURL = *n.ProofURL
	}
	if n.ApproveURL != nil {
		data.ApproveURL = *n.ApproveURL
	}
	if n.RejectURL != nil {
		data.RejectURL = *n.RejectURL
	}
	var body bytes.Buffer
	if err := notificationHTMLTemplate.Execute(&body, data); err != nil {
		return "", "", fmt.Errorf("render notification email: %w", err)
	}
	return body.String(), *n.Message, nil
}

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
