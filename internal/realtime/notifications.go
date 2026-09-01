package realtime

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"socialfund/internal/auth"
	"socialfund/internal/notification"
)

type NotificationHandler struct {
	tokens        *auth.TokenManager
	notifications *notification.Service
	frontendURL   string
	db            *pgxpool.Pool
}

func NewNotificationHandler(tokens *auth.TokenManager, notifications *notification.Service, frontendURL string, db *pgxpool.Pool) *NotificationHandler {
	return &NotificationHandler{tokens: tokens, notifications: notifications, frontendURL: strings.TrimRight(frontendURL, "/"), db: db}
}

func (h *NotificationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := h.tokens.Verify(r.URL.Query().Get("access_token"))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var status string
	if err = h.db.QueryRow(r.Context(), `SELECT status FROM users WHERE id=$1`, identity.UserID).Scan(&status); err != nil || status != "ACTIVE" {
		http.Error(w, "account unavailable", http.StatusForbidden)
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		if h.frontendURL == "" {
			return true
		}
		origin, err := url.Parse(r.Header.Get("Origin"))
		if err != nil {
			return false
		}
		allowed, err := url.Parse(h.frontendURL)
		return err == nil && origin.Scheme == allowed.Scheme && origin.Host == allowed.Host
	}}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(70 * time.Second)) })

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(2 * time.Second)
	ping := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	defer ping.Stop()
	var previous []byte
	push := func() error {
		if err := h.db.QueryRow(r.Context(), `SELECT status FROM users WHERE id=$1`, identity.UserID).Scan(&status); err != nil || status != "ACTIVE" {
			_ = conn.WriteJSON(map[string]any{"type": "account_suspended", "message": "Your account has been suspended. Contact support for help getting back online."})
			return auth.ErrAccountSuspended
		}
		filter := notification.Filter{Limit: 50}
		if identity.Role != "ADMIN" {
			filter.UserID = &identity.UserID
		}
		items, err := h.notifications.List(r.Context(), filter)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(items)
		if bytes.Equal(payload, previous) {
			return nil
		}
		previous = payload
		return conn.WriteJSON(map[string]any{"type": "notifications", "data": items})
	}
	if err = push(); err != nil {
		return
	}
	for {
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if err = push(); err != nil {
				return
			}
		case <-ping.C:
			if err = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}
