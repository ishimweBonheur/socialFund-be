package notification

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io"
	"log/slog"
	"os"
	"socialfund/internal/database"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	mail "github.com/wneessen/go-mail"
)

const testEmailRecipient = "imanaturikumwedidier6@gmail.com"

func sampleData() AccountCreatedEmailData {
	return AccountCreatedEmailData{
		FullName:              "ishimwe bonheur",
		Email:                 "ishimwebonheur078@gmail.com",
		Phone:                 "+250788123456",
		ContributionAmount:    "5,000.00 RWF",
		ContributionFrequency: "Monthly",
		PaymentDue:            "5th of every month",
		LoginURL:              "http://localhost:3000/login",
		Recipient:             testEmailRecipient,
	}
}

func TestAccountCreatedTemplateContainsMemberDataAndLoginButton(t *testing.T) {
	data := sampleData()

	htmlBody, plainBody, err := renderAccountCreated(data)
	if err != nil {
		t.Fatal(err)
	}

	decodedHTML := html.UnescapeString(htmlBody)

	for _, value := range []string{
		data.FullName,
		data.Email,
		data.Phone,
		data.ContributionAmount,
		data.ContributionFrequency,
		data.PaymentDue,
		data.LoginURL,
		"Login to Social Fund",
	} {
		if !strings.Contains(decodedHTML, value) {
			t.Errorf("HTML body missing %q", value)
		}
	}

	for _, color := range []string{
		"#213448",
		"#547792",
		"#94B4C1",
		"#EAE0CF",
	} {
		if !strings.Contains(htmlBody, color) {
			t.Errorf("HTML body missing system palette color %s", color)
		}
	}

	if !strings.Contains(plainBody, data.Email) ||
		!strings.Contains(plainBody, data.LoginURL) {
		t.Fatal("plain body is missing registered email or login URL")
	}
}

func TestBuildLoginURL(t *testing.T) {
	for _, base := range []string{
		"http://localhost:3000",
		"http://localhost:3000/",
	} {
		if got := BuildLoginURL(base); got != "http://localhost:3000/login" {
			t.Fatalf("BuildLoginURL(%q)=%q", base, got)
		}
	}
}

type fakeMailClient struct {
	message string
	err     error
}

func (f *fakeMailClient) DialAndSendWithContext(
	_ context.Context,
	messages ...*mail.Msg,
) error {
	if len(messages) > 0 {
		var buffer bytes.Buffer

		_, _ = messages[0].WriteTo(&buffer)

		f.message = buffer.String()
	}

	return f.err
}

func TestGoMailSenderAddressesNotificationRecipient(t *testing.T) {
	client := &fakeMailClient{}

	sender := &GoMailSender{
		client: client,
		from:   "ishimwebonheur078@gmail.com",
	}

	data := sampleData()

	data.Recipient = testEmailRecipient

	if err := sender.SendAccountCreated(context.Background(), data); err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{
		testEmailRecipient,
		accountCreatedSubject,
		data.Email,
		data.LoginURL,
		"Login to Social Fund",
	} {
		if !strings.Contains(client.message, value) {
			t.Errorf("message missing %q", value)
		}
	}
}

func TestGoMailSenderEmbedsLogoWhenItUsesCID(t *testing.T) {
	client := &fakeMailClient{}
	sender := &GoMailSender{client: client, from: "sender@example.com", logo: []byte("test logo")}
	subject, body := "Contribution overdue", "Please submit your payment proof."

	err := sender.SendNotification(context.Background(), Notification{
		Type:      "CONTRIBUTION_OVERDUE",
		Recipient: "member@example.com",
		Subject:   &subject,
		Message:   &body,
		LogoURL:   embeddedLogoURL,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(client.message, embeddedLogoURL) {
		t.Fatal("email does not contain the logo CID reference")
	}
	if !strings.Contains(client.message, "Content-Id: social-fund-logo") {
		t.Fatal("email does not embed the logo with its expected content ID")
	}
}

type fakeRepository struct {
	items     []Notification
	sent      []uuid.UUID
	failed    []uuid.UUID
	nextRetry time.Time
	data      AccountCreatedEmailData
}

func (f *fakeRepository) Create(
	context.Context,
	database.DBTX,
	Notification,
) (Notification, error) {
	return Notification{}, nil
}

func (f *fakeRepository) ListReady(
	context.Context,
	int,
) ([]Notification, error) {
	return f.items, nil
}

func (f *fakeRepository) MarkSent(
	_ context.Context,
	id uuid.UUID,
) error {
	f.sent = append(f.sent, id)
	return nil
}

func (f *fakeRepository) MarkFailed(
	_ context.Context,
	id uuid.UUID,
	_ string,
	next time.Time,
) error {
	f.failed = append(f.failed, id)
	f.nextRetry = next
	return nil
}

func (f *fakeRepository) LoadAccountCreatedEmailData(
	context.Context,
	uuid.UUID,
) (AccountCreatedEmailData, error) {
	return f.data, nil
}

func (f *fakeRepository) LoadPaymentReminderData(
	context.Context,
	*uuid.UUID,
) (*PaymentReminderData, error) {
	return nil, nil
}

type fakeEmailSender struct {
	data AccountCreatedEmailData
	err  error
}

func (f *fakeEmailSender) SendAccountCreated(
	_ context.Context,
	data AccountCreatedEmailData,
) error {
	f.data = data
	return f.err
}

func (f *fakeEmailSender) SendNotification(
	context.Context,
	Notification,
) error {
	return f.err
}

func TestNotificationTemplateUsesBrandAndStatus(t *testing.T) {
	subject := "Contribution approved"
	message := "Your contribution was approved successfully."

	htmlBody, plainBody, err := renderNotification(
		Notification{
			Type:    "CONTRIBUTION_APPROVED",
			Subject: &subject,
			Message: &message,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{
		"#213448",
		"SUCCESS",
		subject,
		message,
	} {
		if !strings.Contains(htmlBody, value) {
			t.Errorf("HTML body missing %q", value)
		}
	}

	if plainBody != message {
		t.Fatalf("plain body=%q", plainBody)
	}
}

func TestPaymentReminderTemplateContainsPaymentInstructions(t *testing.T) {
	subject := "Contribution due soon"

	message := "Your contribution is due in 3 days. Please pay before the due date to avoid a late fee."

	htmlBody, _, err := renderNotification(
		Notification{
			Type:       "CONTRIBUTION_DUE",
			Subject:    &subject,
			Message:    &message,
			PaymentURL: "https://example.com/dashboard",

			Reminder: &PaymentReminderData{
				MemberName:   "Patience Ineza",
				AmountDue:    "20,000.00 RWF",
				DueDate:      "7 September 2026",
				AccountName:  "Social Fund",
				PaymentType:  "MERCHANT",
				MerchantCode: "123456",
				USSDCode:     "*182*8*1*123456#",
				QRCodeData:   "cid:social-fund-payment-qr",
			},
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	decoded := html.UnescapeString(htmlBody)

	for _, value := range []string{
		"Hello Patience Ineza",
		"20,000.00 RWF",
		"7 September 2026",
		"Social Fund",
		"123456",
		"*182*8*1*123456#",
		"Scan to dial payment",
		"Add payment details and upload proof",
	} {
		if !strings.Contains(decoded, value) {
			t.Errorf("HTML body missing %q", value)
		}
	}
}

type unusedSender struct{}

func (unusedSender) Send(
	context.Context,
	Notification,
) error {
	return nil
}

func TestWorkerMarksAccountCreatedSent(t *testing.T) {
	id := uuid.New()

	repo := &fakeRepository{
		items: []Notification{
			{
				ID:        id,
				UserID:    uuid.New(),
				Type:      "ACCOUNT_CREATED",
				Recipient: "member@example.com",
				Attempts:  1,
			},
		},
		data: sampleData(),
	}

	email := &fakeEmailSender{}

	routing := NewRoutingSender(
		unusedSender{},
		repo,
		email,
		"http://localhost:3000/",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	worker := NewWorker(
		NewService(repo),
		routing,
	)

	if _, err := worker.RunBatch(context.Background(), 10); err != nil {
		t.Fatal(err)
	}

	if len(repo.sent) != 1 || repo.sent[0] != id {
		t.Fatal("notification was not marked SENT")
	}

	if email.data.Recipient != "member@example.com" ||
		email.data.LoginURL != "http://localhost:3000/login" {
		t.Fatalf("sender data=%+v", email.data)
	}
}

func TestWorkerMarksAccountCreatedFailedWithRetry(t *testing.T) {
	id := uuid.New()

	repo := &fakeRepository{
		items: []Notification{
			{
				ID:        id,
				UserID:    uuid.New(),
				Type:      "ACCOUNT_CREATED",
				Recipient: "member@example.com",
				Attempts:  1,
			},
		},
		data: sampleData(),
	}

	email := &fakeEmailSender{
		err: errors.New("SMTP unavailable"),
	}

	routing := NewRoutingSender(
		unusedSender{},
		repo,
		email,
		"http://localhost:3000",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	worker := NewWorker(
		NewService(repo),
		routing,
	)

	before := time.Now()

	if _, err := worker.RunBatch(context.Background(), 10); err != nil {
		t.Fatal(err)
	}

	if len(repo.failed) != 1 || repo.failed[0] != id {
		t.Fatal("notification was not marked FAILED")
	}

	if !repo.nextRetry.After(before) {
		t.Fatal("retry time was not preserved")
	}
}

// TestGmailIntegration sends a real email through Gmail SMTP.
//
// It always sends the test email to:
// imanaturikumwedidier6@gmail.com
//
// Required .env variables:
//
// SMTP_USERNAME=your-gmail@gmail.com
// SMTP_PASSWORD=your-google-app-password
// SMTP_FROM=your-gmail@gmail.com
//
// Gmail host and port are automatically set to:
// smtp.gmail.com:587
func TestGmailIntegration(t *testing.T) {
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")

	if username == "" || password == "" || from == "" {
		t.Skip(
			"SMTP_USERNAME, SMTP_PASSWORD or SMTP_FROM is not configured",
		)
	}

	// Gmail SMTP defaults.
	host := "smtp.gmail.com"
	port := 587

	sender, err := NewGoMailSender(
		host,
		port,
		username,
		password,
		from,
	)
	if err != nil {
		t.Fatal(err)
	}

	data := sampleData()

	// IMPORTANT:
	// The actual test email is always delivered to this address.
	data.Recipient = testEmailRecipient

	// Keep the member information in the email itself.
	// This allows you to test how the email looks to a member.
	data.Email = "ishimwebonheur078@gmail.com"

	if err := sender.SendAccountCreated(
		context.Background(),
		data,
	); err != nil {
		t.Fatalf(
			"failed to send test email to %s: %v",
			testEmailRecipient,
			err,
		)
	}

	t.Logf(
		"Test email sent successfully to %s",
		testEmailRecipient,
	)
}
