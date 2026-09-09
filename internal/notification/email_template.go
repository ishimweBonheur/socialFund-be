package notification

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"
	texttemplate "text/template"
	"unicode"

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
	LogoURL               template.URL
	Heading               string
	Intro                 string
}

func BuildLoginURL(frontendURL string) string {
	return strings.TrimRight(frontendURL, "/") + "/login"
}

var htmlTemplate = template.Must(template.New("account-created").Parse(`
<!doctype html>
<html>
  <body style="margin:0;background:#EAE0CF;font-family:Arial,sans-serif;color:#213448">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
      <tr>
        <td style="padding:32px 12px">
          <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:600px;margin:auto;background:#EAE0CF;border-radius:12px;overflow:hidden;box-shadow:0 18px 48px #94B4C1">
            <tr>
              <td style="padding:24px 30px;background:#213448;color:#EAE0CF;font-size:22px;font-weight:bold">
                {{if .LogoURL}}<img src="{{.LogoURL}}" width="48" height="48" alt="Social Fund" style="display:inline-block;vertical-align:middle;margin-right:12px">{{else}}<span style="display:inline-block;width:42px;height:42px;line-height:42px;text-align:center;vertical-align:middle;margin-right:12px;border:2px solid #EAE0CF;border-radius:50%;color:#F5C978;font-size:16px;font-weight:bold">SF</span>{{end}}Social Fund
                <br>
                <span style="font-size:11px;font-weight:normal;color:#94B4C1">Community finance</span>
              </td>
            </tr>
            <tr>
              <td style="padding:32px 30px">
                <div style="display:inline-block;padding:6px 10px;border-radius:6px;background:#94B4C1;color:#213448;font-size:11px;font-weight:bold;letter-spacing:1px">
                  ACCOUNT CREATED
                </div>
                <h1 style="font-size:24px;margin:20px 0 8px">{{.Heading}}</h1>
                <p style="color:#547792;line-height:1.6;margin-top:0">
                  {{.Intro}}
                </p>

                <h2 style="font-size:15px;margin-top:28px">Account details</h2>

                <table role="presentation" width="100%" cellspacing="0" cellpadding="11" style="background:#94B4C1;border-radius:9px;box-shadow:0 8px 24px #94B4C1">
                  <tr>
                    <td><strong>Name</strong></td>
                    <td>{{.FullName}}</td>
                  </tr>
                  <tr>
                    <td><strong>Email</strong></td>
                    <td>{{.Email}}</td>
                  </tr>
                  <tr>
                    <td><strong>Phone</strong></td>
                    <td>{{.Phone}}</td>
                  </tr>
                  <tr>
                    <td><strong>Contribution amount</strong></td>
                    <td>{{.ContributionAmount}}</td>
                  </tr>
                  <tr>
                    <td><strong>Contribution frequency</strong></td>
                    <td>{{.ContributionFrequency}}</td>
                  </tr>
                  <tr>
                    <td><strong>Payment due</strong></td>
                    <td>{{.PaymentDue}}</td>
                  </tr>
                </table>

                <p style="margin-top:26px">
                  <strong>Your account is currently inactive.</strong>
                </p>
                <p style="color:#547792;line-height:1.6">
                  Sign in with your registered Google account ({{.Email}}) to verify and activate it.
                </p>
                <p style="margin:28px 0 8px">
                  <a href="{{.LoginURL}}" style="display:inline-block;background:#547792;color:#EAE0CF;text-decoration:none;padding:13px 22px;border-radius:8px;font-weight:bold;box-shadow:0 8px 18px #94B4C1">
                    Login to Social Fund
                  </a>
                </p>
              </td>
            </tr>
            <tr>
              <td style="padding:18px 30px;background:#94B4C1;color:#213448;font-size:12px">
                This is an automated message from Social Fund.
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>
`))

type notificationEmailData struct {
	Subject, Heading, Label, Accent, Tint string
	LogoURL                               template.URL
	Body                                  template.HTML
	ApproveURL, RejectURL                 string
	PaymentURL                            string
	Reminder                              *PaymentReminderData
	QRCodeData                            template.URL
}

var urlPattern = regexp.MustCompile(`https?://[^\s<]+`)

var notificationHTMLTemplate = template.Must(template.New("notification").Parse(`
<!doctype html>
<html>
  <body style="margin:0;background:#EAE0CF;font-family:Arial,sans-serif;color:#213448">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
      <tr>
        <td style="padding:32px 12px">
          <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:600px;margin:auto;background:#EAE0CF;border-radius:12px;overflow:hidden;box-shadow:0 18px 48px #94B4C1">
            <!-- Header -->
            <tr>
              <td style="padding:24px 30px;background:#213448;color:#EAE0CF;font-size:22px;font-weight:bold">
                {{if .LogoURL}}<img src="{{.LogoURL}}" width="48" height="48" alt="Social Fund" style="display:inline-block;vertical-align:middle;margin-right:12px">{{else}}<span style="display:inline-block;width:42px;height:42px;line-height:42px;text-align:center;vertical-align:middle;margin-right:12px;border:2px solid #EAE0CF;border-radius:50%;color:#F5C978;font-size:16px;font-weight:bold">SF</span>{{end}}Social Fund
                <br>
                <span style="font-size:11px;font-weight:normal;color:#94B4C1">Community finance</span>
              </td>
            </tr>
            <!-- Body -->
            <tr>
              <td style="padding:32px 30px">
                <!-- Label Badge -->
                <div style="display:inline-block;padding:6px 10px;border-radius:6px;background:{{.Tint}};color:{{.Accent}};font-size:11px;font-weight:bold;letter-spacing:1px">
                  {{.Label}}
                </div>
                <!-- Subject -->
                <h1 style="font-size:24px;line-height:1.3;margin:20px 0">{{.Heading}}</h1>
                {{if .Reminder}}
                  <p style="font-size:18px;margin:0 0 12px">Hello {{.Reminder.MemberName}},</p>
                  {{if eq .Label "PAYMENT OVERDUE"}}
                  <p style="color:#213448;line-height:1.6;margin-top:0">Your contribution of {{.Reminder.TotalAmountDue}} was due on {{.Reminder.DueDate}} and has not yet been recorded as paid.</p>
                  <p style="color:#213448;line-height:1.6">Please make your payment as soon as possible.</p>
                  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#FFF7E8;border:1px solid #F0D39A;border-radius:9px;margin:20px 0">
                    <tr><td style="padding:14px 18px"><strong>Original amount</strong><br>{{.Reminder.OriginalAmount}}</td><td style="padding:14px 18px"><strong>Late fee</strong><br>{{if eq .Reminder.LateFee "0.00 RWF"}}None{{else}}{{.Reminder.LateFee}}{{end}}</td></tr>
                    <tr><td style="padding:14px 18px"><strong>Total amount due</strong><br><strong>{{.Reminder.TotalAmountDue}}</strong></td><td style="padding:14px 18px"><strong>Days overdue</strong><br>{{.Reminder.DaysOverdue}} days</td></tr>
                  </table>
                  {{else}}
                  <p style="color:#213448;line-height:1.6;margin-top:0">{{.Body}}</p>
                  {{end}}
                  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#F3F7FA;border:1px solid #D9E6EF;border-radius:9px;margin:24px 0">
                    <tr>
                      <td style="padding:18px 20px;border-right:1px solid #D9E6EF;width:50%"><div style="color:#2876B2;font-size:12px;font-weight:bold;text-transform:uppercase">Amount due</div><strong style="display:block;font-size:24px;margin-top:6px">{{.Reminder.AmountDue}}</strong></td>
                      <td style="padding:18px 20px"><div style="color:#2876B2;font-size:12px;font-weight:bold;text-transform:uppercase">Due date</div><strong style="display:block;font-size:20px;margin-top:8px">{{.Reminder.DueDate}}</strong></td>
                    </tr>
                  </table>
                  <div style="border:1px solid #E3E3E3;border-radius:9px;padding:20px;margin-bottom:18px">
                    <h2 style="font-size:17px;margin:0 0 16px">How to pay</h2>
                    <p style="margin:8px 0"><strong>Pay to:</strong> {{.Reminder.AccountName}}</p>
                    <p style="margin:8px 0"><strong>Payment method:</strong> {{.Reminder.PaymentMethodLabel}}</p>
                    {{if eq .Reminder.PaymentType "PHONE"}}<p style="margin:8px 0"><strong>Phone number:</strong> {{.Reminder.PhoneNumber}}</p>{{end}}
                    {{if eq .Reminder.PaymentType "MERCHANT"}}<p style="margin:8px 0"><strong>Merchant code:</strong> {{.Reminder.MerchantCode}}</p>{{end}}
                    <p style="margin:8px 0"><strong>USSD code:</strong> {{.Reminder.USSDCode}}</p>
                    {{if .QRCodeData}}<div style="margin:18px 0 4px;text-align:center"><div style="color:#2876B2;font-weight:bold;margin-bottom:8px">Scan to dial payment</div><img src="{{.QRCodeData}}" width="220" height="220" alt="Scan to dial payment" style="display:inline-block;border:1px solid #D9E6EF;padding:8px;background:#fff"></div>{{end}}
                  </div>
                  {{if .PaymentURL}}<p style="margin:0"><a href="{{.PaymentURL}}" style="display:block;text-align:center;background:#2876B2;color:#fff;text-decoration:none;padding:13px 18px;border-radius:7px;font-weight:bold">Add payment details and upload proof</a></p>{{end}}
                {{else}}
                  <div style="background:#94B4C1;border-radius:9px;padding:20px;color:#213448;font-size:15px;line-height:1.7;box-shadow:0 8px 24px #94B4C1">
                    {{.Body}}
                  </div>
                {{end}}
                <!-- Action Buttons (conditional) -->
                {{if .ApproveURL}}
                  <table role="presentation" cellspacing="0" cellpadding="0" style="margin-top:14px">
                    <tr>
                      <td style="padding-right:10px">
                        <a href="{{.ApproveURL}}" style="display:inline-block;background:#547792;color:#EAE0CF;text-decoration:none;padding:12px 20px;border-radius:8px;font-weight:bold">
                          Approve
                        </a>
                      </td>
                      <td>
                        <a href="{{.RejectURL}}" style="display:inline-block;background:#213448;color:#EAE0CF;text-decoration:none;padding:12px 20px;border-radius:8px;font-weight:bold">
                          Reject
                        </a>
                      </td>
                    </tr>
                  </table>
                {{end}}
              </td>
            </tr>
            <!-- Footer -->
            <tr>
              <td style="padding:18px 30px;background:#94B4C1;color:#213448;font-size:12px">
                This is an automated message from Social Fund. Please keep transaction references for your records.
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>
`))

func notificationAppearance(kind string) (string, string, string) {
	switch kind {
	case "CONTRIBUTION_APPROVED":
		return "SUCCESS", "#213448", "#94B4C1"
	case "CONTRIBUTION_REJECTED":
		return "ACTION REQUIRED", "#213448", "#94B4C1"
	case "CONTRIBUTION_OVERDUE":
		return "PAYMENT OVERDUE", "#547792", "#94B4C1"
	case "ADMIN_CONTRIBUTION_OVERDUE":
		return "MEMBER FOLLOW-UP", "#547792", "#94B4C1"
	case "PROOF_SUBMITTED":
		return "REVIEW REQUIRED", "#547792", "#94B4C1"
	default:
		return "NOTIFICATION", "#547792", "#94B4C1"
	}
}

func renderNotification(n Notification) (string, string, error) {
	if n.Subject == nil || n.Message == nil {
		return "", "", fmt.Errorf("notification content is missing")
	}
	label, accent, tint := notificationAppearance(n.Type)
	subject, heading, message := *n.Subject, *n.Subject, *n.Message
	if n.Reminder != nil {
		if n.Type == "CONTRIBUTION_OVERDUE" {
			heading = fmt.Sprintf("Your contribution is overdue  %d days late", n.Reminder.DaysOverdue)
			label = "PAYMENT OVERDUE"
		} else {
			heading = "Your contribution is almost due"
			if n.Reminder.DaysUntilDue == 1 {
				label = "DUE TOMORROW"
			} else if n.Reminder.DaysUntilDue > 1 {
				label = fmt.Sprintf("DUE IN %d DAYS", n.Reminder.DaysUntilDue)
			} else {
				label = "PAYMENT DUE"
			}
		}
	}
	escaped := html.EscapeString(message)
	escaped = urlPattern.ReplaceAllStringFunc(escaped, func(value string) string {
		return `<a href="` + value + `" style="color:#547792;font-weight:bold;word-break:break-all">` + value + `</a>`
	})
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")

	data := notificationEmailData{
		Subject:    subject,
		Heading:    heading,
		Label:      label,
		Accent:     accent,
		Tint:       tint,
		Body:       template.HTML(escaped),
		LogoURL:    template.URL(n.LogoURL),
		PaymentURL: n.PaymentURL,
		Reminder:   n.Reminder,
	}
	if n.Reminder != nil {
		reminder := *n.Reminder
		reminder.MemberName = firstName(reminder.MemberName)
		data.Reminder = &reminder
		data.QRCodeData = template.URL(n.Reminder.QRCodeData)
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
	return body.String(), message, nil
}

func firstName(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return ""
	}
	name := []rune(strings.ToLower(parts[0]))
	name[0] = unicode.ToUpper(name[0])
	return string(name)
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

Login to Social Fund:
{{.LoginURL}}

Your account will become active after your registered Google account is successfully verified.`))

func renderAccountCreated(data AccountCreatedEmailData) (string, string, error) {
	data.Heading = "Welcome, " + data.FullName
	data.Intro = "Your Social Fund account has been created successfully."

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
