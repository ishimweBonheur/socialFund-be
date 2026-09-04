package paymentconfig

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/httpx"
)

type Settings struct {
	AccountName  string     `json:"account_name"`
	PaymentType  string     `json:"payment_type"`
	PhoneNumber  *string    `json:"phone_number,omitempty"`
	MerchantCode *string    `json:"merchant_code,omitempty"`
	USSDTemplate string     `json:"ussd_template"`
	USSDCode     string     `json:"ussd_code"`
	UpdatedBy    *uuid.UUID `json:"updated_by,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }
func (r *Repository) Get(ctx context.Context) (Settings, error) {
	var s Settings
	err := r.db.QueryRow(ctx, `SELECT account_name,payment_type,phone_number,merchant_code,ussd_template,updated_by,updated_at FROM payment_settings WHERE id=1`).Scan(&s.AccountName, &s.PaymentType, &s.PhoneNumber, &s.MerchantCode, &s.USSDTemplate, &s.UpdatedBy, &s.UpdatedAt)
	s.USSDCode = format(s)
	return s, err
}
func (r *Repository) Update(ctx context.Context, s Settings) (Settings, error) {
	err := r.db.QueryRow(ctx, `UPDATE payment_settings SET account_name=$1,payment_type=$2,phone_number=$3,merchant_code=$4,ussd_template=$5,updated_by=$6,updated_at=NOW() WHERE id=1 RETURNING updated_at`, s.AccountName, s.PaymentType, s.PhoneNumber, s.MerchantCode, s.USSDTemplate, s.UpdatedBy).Scan(&s.UpdatedAt)
	s.USSDCode = format(s)
	return s, err
}
func format(s Settings) string {
	v := s.USSDTemplate
	if s.PhoneNumber != nil {
		v = strings.ReplaceAll(v, "{phone_number}", *s.PhoneNumber)
	}
	if s.MerchantCode != nil {
		v = strings.ReplaceAll(v, "{merchant_code}", *s.MerchantCode)
	}
	return v
}

var validUSSD = regexp.MustCompile(`^[*#0-9{}a-z_]+$`)

type Service struct{ repo *Repository }

func NewService(r *Repository) *Service                      { return &Service{repo: r} }
func (s *Service) Get(ctx context.Context) (Settings, error) { return s.repo.Get(ctx) }
func (s *Service) Update(ctx context.Context, admin uuid.UUID, in Settings) (Settings, error) {
	in.AccountName = strings.TrimSpace(in.AccountName)
	in.PaymentType = strings.ToUpper(strings.TrimSpace(in.PaymentType))
	in.USSDTemplate = strings.TrimSpace(in.USSDTemplate)
	valid := in.AccountName != "" && len(in.AccountName) <= 120 && len(in.USSDTemplate) <= 200 && validUSSD.MatchString(in.USSDTemplate)
	if in.PaymentType == "PHONE" {
		valid = valid && in.PhoneNumber != nil && strings.Contains(in.USSDTemplate, "{phone_number}")
		in.MerchantCode = nil
	} else if in.PaymentType == "MERCHANT" {
		valid = valid && in.MerchantCode != nil && strings.Contains(in.USSDTemplate, "{merchant_code}")
		in.PhoneNumber = nil
	} else {
		valid = false
	}
	if !valid {
		return Settings{}, httpx.ErrValidation
	}
	in.UpdatedBy = &admin
	return s.repo.Update(ctx, in)
}

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(s *Service, l *slog.Logger) *Handler { return &Handler{service: s, logger: l} }
func (h *Handler) MemberRoutes() chi.Router          { r := chi.NewRouter(); r.Get("/", h.get); return r }
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.get)
	r.Put("/", h.update)
	return r
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	v, e := h.service.Get(r.Context())
	if e != nil {
		httpx.WriteInternal(w, r, h.logger, "get_payment_settings", e)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": v})
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	a, _ := httpx.IdentityFrom(r.Context())
	var in Settings
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	v, e := h.service.Update(r.Context(), a.UserID, in)
	if errors.Is(e, httpx.ErrValidation) {
		httpx.WriteError(w, httpx.ErrValidation)
		return
	}
	if e != nil {
		httpx.WriteInternal(w, r, h.logger, "update_payment_settings", e)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": v})
}
