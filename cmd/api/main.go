package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"socialfund/internal/assistance"
	"socialfund/internal/audit"
	"socialfund/internal/config"
	"socialfund/internal/contribution"
	"socialfund/internal/contributionplan"
	"socialfund/internal/database"
	"socialfund/internal/fund"
	"socialfund/internal/notification"
	"socialfund/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	userRepo := user.NewRepository(pool)
	planRepo := contributionplan.NewRepository(pool)
	contributionRepo := contribution.NewRepository(pool)
	assistanceRepo := assistance.NewRepository(pool)
	fundRepo := fund.NewRepository()
	auditRepo := audit.NewRepository()
	notificationRepo := notification.NewRepository(pool)

	userService := user.NewService(userRepo)
	planService := contributionplan.NewService(planRepo)
	contributionService := contribution.NewService(pool, contributionRepo, fundRepo, auditRepo, notificationRepo, userRepo)
	assistanceService := assistance.NewService(pool, assistanceRepo, fundRepo, auditRepo, notificationRepo)
	_ = notification.NewService(notificationRepo)

	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	router.Get("/healthz", database.HealthHandler(pool))
	router.Route("/api/v1", func(r chi.Router) {
		r.Mount("/users", user.NewHandler(userService).Routes())
		r.Mount("/contribution-plans", contributionplan.NewHandler(planService).Routes())
		r.Mount("/contributions", contribution.NewHandler(contributionService).Routes())
		r.Mount("/assistance-requests", assistance.NewHandler(assistanceService).Routes())
	})

	server := &http.Server{Addr: cfg.HTTPAddress, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("API listening on %s", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server failed: %v", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown failed: %v", err)
	}
}
