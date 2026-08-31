package notification

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	mail "github.com/wneessen/go-mail"
	"socialfund/internal/database"
)

func sampleData() AccountCreatedEmailData {
	return AccountCreatedEmailData{FullName: "Patience Ineza", Email: "patience@gmail.com", Phone: "+250788123456", ContributionAmount: "5,000.00 RWF", ContributionFrequency: "Monthly", PaymentDue: "5th of every month", LoginURL: "http://localhost:3000/login", Recipient: "patience@gmail.com"}
}

func TestAccountCreatedTemplateContainsMemberDataAndLoginButton(t *testing.T) {
	data := sampleData()
	htmlBody, plainBody, err := renderAccountCreated(data)
	if err != nil {
		t.Fatal(err)
	}
	decodedHTML := html.UnescapeString(htmlBody)
	for _, value := range []string{data.FullName, data.Email, data.Phone, data.ContributionAmount, data.ContributionFrequency, data.PaymentDue, data.LoginURL, "Login to Social Fund"} {
		if !strings.Contains(decodedHTML, value) {
			t.Errorf("HTML body missing %q", value)
		}
	}
	for _, color := range []string{"#213448", "#547792", "#94B4C1", "#EAE0CF"} {
		if !strings.Contains(htmlBody, color) {
			t.Errorf("HTML body missing system palette color %s", color)
		}
	}
	if !strings.Contains(plainBody, data.Email) || !strings.Contains(plainBody, data.LoginURL) {
		t.Fatal("plain body is missing registered email or login URL")
	}
}
func TestBuildLoginURL(t *testing.T) {
	for _, base := range []string{"http://localhost:3000", "http://localhost:3000/"} {
		if got := BuildLoginURL(base); got != "http://localhost:3000/login" {
			t.Fatalf("BuildLoginURL(%q)=%q", base, got)
		}
	}
}

type fakeMailClient struct {
	message string
	err     error
}

func (f *fakeMailClient) DialAndSendWithContext(_ context.Context, messages ...*mail.Msg) error {
	if len(messages) > 0 {
		var buffer bytes.Buffer
		_, _ = messages[0].WriteTo(&buffer)
		f.message = buffer.String()
	}
	return f.err
}
func TestGoMailSenderAddressesNotificationRecipient(t *testing.T) {
	client := &fakeMailClient{}
	sender := &GoMailSender{client: client, from: "fund@example.com"}
	data := sampleData()
	data.Recipient = "delivery@example.com"
	if err := sender.SendAccountCreated(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"delivery@example.com", accountCreatedSubject, data.Email, data.LoginURL, "Login to Social Fund"} {
		if !strings.Contains(client.message, value) {
			t.Errorf("message missing %q", value)
		}
	}
}

type fakeRepository struct {
	items     []Notification
	sent      []uuid.UUID
	failed    []uuid.UUID
	nextRetry time.Time
	data      AccountCreatedEmailData
}

func (f *fakeRepository) Create(context.Context, database.DBTX, Notification) (Notification, error) {
	return Notification{}, nil
}
func (f *fakeRepository) ListReady(context.Context, int) ([]Notification, error) {
	return f.items, nil
}
func (f *fakeRepository) MarkSent(_ context.Context, id uuid.UUID) error {
	f.sent = append(f.sent, id)
	return nil
}
func (f *fakeRepository) MarkFailed(_ context.Context, id uuid.UUID, _ string, next time.Time) error {
	f.failed = append(f.failed, id)
	f.nextRetry = next
	return nil
}
func (f *fakeRepository) LoadAccountCreatedEmailData(context.Context, uuid.UUID) (AccountCreatedEmailData, error) {
	return f.data, nil
}

type fakeEmailSender struct {
	data AccountCreatedEmailData
	err  error
}

func (f *fakeEmailSender) SendAccountCreated(_ context.Context, data AccountCreatedEmailData) error {
	f.data = data
	return f.err
}
func (f *fakeEmailSender) SendNotification(context.Context, Notification) error {
	return f.err
}

func TestNotificationTemplateUsesBrandAndStatus(t *testing.T) {
	subject, message := "Contribution approved", "Your contribution was approved successfully."
	htmlBody, plainBody, err := renderNotification(Notification{Type: "CONTRIBUTION_APPROVED", Subject: &subject, Message: &message})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"#213448", "SUCCESS", subject, message} {
		if !strings.Contains(htmlBody, value) {
			t.Errorf("HTML body missing %q", value)
		}
	}
	if plainBody != message {
		t.Fatalf("plain body=%q", plainBody)
	}
}

type unusedSender struct{}

func (unusedSender) Send(context.Context, Notification) error {
	return nil
}
func TestWorkerMarksAccountCreatedSent(t *testing.T) {
	id := uuid.New()
	repo := &fakeRepository{items: []Notification{{ID: id, UserID: uuid.New(), Type: "ACCOUNT_CREATED", Recipient: "member@example.com", Attempts: 1}}, data: sampleData()}
	email := &fakeEmailSender{}
	routing := NewRoutingSender(unusedSender{}, repo, email, "http://localhost:3000/", slog.New(slog.NewTextHandler(io.Discard, nil)))
	worker := NewWorker(NewService(repo), routing)
	if _, err := worker.RunBatch(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if len(repo.sent) != 1 || repo.sent[0] != id {
		t.Fatal("notification was not marked SENT")
	}
	if email.data.Recipient != "member@example.com" || email.data.LoginURL != "http://localhost:3000/login" {
		t.Fatalf("sender data=%+v", email.data)
	}
}
func TestWorkerMarksAccountCreatedFailedWithRetry(t *testing.T) {
	id := uuid.New()
	repo := &fakeRepository{items: []Notification{{ID: id, UserID: uuid.New(), Type: "ACCOUNT_CREATED", Recipient: "member@example.com", Attempts: 1}}, data: sampleData()}
	email := &fakeEmailSender{err: errors.New("SMTP unavailable")}
	routing := NewRoutingSender(unusedSender{}, repo, email, "http://localhost:3000", slog.New(slog.NewTextHandler(io.Discard, nil)))
	worker := NewWorker(NewService(repo), routing)
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

func TestGmailIntegration(t *testing.T) {
	host, user, password, from := os.Getenv("SMTP_HOST"), os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD"), os.Getenv("SMTP_FROM")
	if host == "" || user == "" || password == "" || from == "" {
		t.Skip("SMTP credentials not configured")
	}
	port, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		t.Fatal(err)
	}
	sender, err := NewGoMailSender(host, port, user, password, from)
	if err != nil {
		t.Fatal(err)
	}
	data := sampleData()
	data.Email = user
	data.Recipient = user
	if err = sender.SendAccountCreated(context.Background(), data); err != nil {
		t.Fatal(err)
	}
}
