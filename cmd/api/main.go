package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"socialfund/internal/assistance"
	"socialfund/internal/audit"
	"socialfund/internal/auth"
	"socialfund/internal/config"
	"socialfund/internal/contribution"
	"socialfund/internal/contributionplan"
	"socialfund/internal/database"
	"socialfund/internal/docs"
	"socialfund/internal/fund"
	"socialfund/internal/httpx"
	"socialfund/internal/notification"
	"socialfund/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}
	var logHandler slog.Handler
	if cfg.AppEnv == "production" {
		logHandler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, nil)
	}
	logger := slog.New(logHandler).With("service", "social-fund")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, cfg)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	userRepo := user.NewRepository(pool)
	planRepo := contributionplan.NewRepository(pool)
	contributionRepo := contribution.NewRepository(pool)
	assistanceRepo := assistance.NewRepository(pool)
	fundRepo := fund.NewRepository()
	auditRepo := audit.NewRepository()
	notificationRepo := notification.NewRepository(pool)
	userService := user.NewService(pool, userRepo, planRepo, notificationRepo, auditRepo, cfg.FrontendURL)
	planService := contributionplan.NewService(planRepo)
	contributionService := contribution.NewService(pool, contributionRepo, fundRepo, auditRepo, notificationRepo, userRepo)
	assistanceService := assistance.NewService(pool, assistanceRepo, fundRepo, auditRepo, notificationRepo)
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiration)
	authService := auth.NewService(pool, userRepo, auditRepo, auth.NewGoogleVerifier(cfg.GoogleClientID), tokenManager, logger)
	notificationService := notification.NewService(notificationRepo)
	if cfg.SMTPHost != "" && cfg.SMTPUsername != "" && cfg.SMTPPassword != "" {
		port, parseErr := strconv.Atoi(cfg.SMTPPort)
		if parseErr != nil {
			logger.Error("invalid SMTP port", "error", parseErr)
			os.Exit(1)
		}
		emailSender, senderErr := notification.NewGoMailSender(cfg.SMTPHost, port, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
		if senderErr != nil {
			logger.Error("configure email sender", "error", senderErr)
			os.Exit(1)
		}
		fallback := notification.NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
		routingSender := notification.NewRoutingSender(fallback, notificationRepo, emailSender, cfg.FrontendURL, logger)
		worker := notification.NewWorker(notificationService, routingSender)
		go func() {
			if err := worker.Run(ctx, 10*time.Second, 100); err != nil {
				logger.Error("notification worker stopped", "error", err)
				stop()
			}
		}()
	} else if cfg.SMTPHost != "" {
		logger.Warn("notification worker disabled because SMTP credentials are incomplete")
	}
	go func() {
		if err := contributionService.RunOverdueScheduler(ctx, time.Hour, 100); err != nil {
			logger.Error("overdue scheduler stopped", "error", err)
			stop()
		}
	}()

	router := chi.NewRouter()
	router.Use(middleware.Recoverer, httpx.RequestIDMiddleware, httpx.LoggingMiddleware(logger))
	router.Get("/healthz", database.HealthHandler(pool))
	if cfg.AppEnv != "production" {
		router.Get("/swagger/openapi.yaml", docs.Spec)
		router.Get("/swagger/index.html", docs.UI)
		router.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/swagger/index.html", http.StatusTemporaryRedirect)
		})
	}
	router.Route("/api/v1", func(r chi.Router) {
		r.Mount("/auth", auth.NewHandler(authService, logger).Routes())
		r.Group(func(protected chi.Router) {
			protected.Use(auth.Authenticate(tokenManager))
			protected.Mount("/users", user.NewHandler(userService, logger).Routes())
			protected.Group(func(adminOnly chi.Router) {
				adminOnly.Use(auth.RequireAdmin)
				adminOnly.Mount("/contribution-plans", contributionplan.NewHandler(planService).Routes())
				adminOnly.Mount("/contributions", contribution.NewHandler(contributionService, logger).Routes())
				adminOnly.Mount("/assistance-requests", assistance.NewHandler(assistanceService, logger).Routes())
			})
		})
		r.Route("/admin", func(admin chi.Router) {
			admin.Use(auth.Authenticate(tokenManager), auth.RequireAdmin)
			admin.Mount("/users", user.NewHandler(userService, logger).AdminRoutes())
		})
	})
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		logger.Info("API listening", "address", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP shutdown failed", "error", err)
	}
}
