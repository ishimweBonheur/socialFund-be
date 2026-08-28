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
	"socialfund/internal/dashboard"
	"socialfund/internal/database"
	"socialfund/internal/docs"
	"socialfund/internal/fund"
	"socialfund/internal/httpx"
	appmetrics "socialfund/internal/metrics"
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
	fundRepo := fund.NewRepository(pool)
	auditRepo := audit.NewRepository(pool)
	notificationRepo := notification.NewRepository(pool)
	userService := user.NewService(pool, userRepo, planRepo, notificationRepo, auditRepo, cfg.FrontendURL)
	planService := contributionplan.NewService(planRepo, pool, auditRepo)
	contributionService := contribution.NewService(pool, contributionRepo, fundRepo, auditRepo, notificationRepo, userRepo, cfg.FrontendURL, cfg.APIPublicURL)
	assistanceService := assistance.NewService(pool, assistanceRepo, fundRepo, auditRepo, notificationRepo)
	var proofStorage contribution.FileStorage
	if cfg.StorageDriver == "s3" {
		proofStorage, err = contribution.NewS3Storage(ctx, cfg.S3Endpoint, cfg.S3Region, cfg.S3Bucket, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3UsePathStyle)
		if err != nil {
			logger.Error("configure proof storage", "error", err)
			os.Exit(1)
		}
	} else {
		proofStorage = contribution.NewLocalFileStorage(cfg.StorageLocalPath+"/proofs", "/uploads/proofs")
	}
	contributionHandler := contribution.NewHandler(contributionService, proofStorage, logger)
	assistanceHandler := assistance.NewHandler(assistanceService, logger)
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiration)
	authService := auth.NewService(pool, userRepo, auditRepo, auth.NewGoogleVerifier(cfg.GoogleClientID), tokenManager, logger)
	notificationService := notification.NewService(notificationRepo, pool, auditRepo)
	dashboardHandler := dashboard.NewHandler(pool, logger)
	metricsRegistry := appmetrics.New(pool)
	generalLimiter := httpx.NewRateLimiter(cfg.RateLimitRPM)
	authLimiter := httpx.NewRateLimiter(cfg.AuthRateLimitRPM)
	if cfg.RateLimitEnabled {
		go generalLimiter.Cleanup(ctx.Done())
		go authLimiter.Cleanup(ctx.Done())
	}
	if cfg.SMTPHost != "" && cfg.SMTPUsername != "" && cfg.SMTPPassword != "" {
		port, parseErr := strconv.Atoi(cfg.SMTPPort)
		if parseErr != nil {
			logger.Error("invalid SMTP port", "error", parseErr)
			os.Exit(1)
		}
		emailSender, senderErr := notification.NewGoMailSender(cfg.SMTPHost, port, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom, proofStorage)
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
	router.Use(middleware.Recoverer, httpx.CORS(cfg.FrontendURL), httpx.RequestIDMiddleware, metricsRegistry.Middleware, httpx.LoggingMiddleware(logger))
	if cfg.RateLimitEnabled {
		router.Use(generalLimiter.Middleware)
	}
	router.Get("/healthz", database.HealthHandler(pool))
	if cfg.AppEnv != "production" {
		router.Get("/metrics", metricsRegistry.Handler)
	} else {
		router.With(auth.Authenticate(tokenManager), auth.RequireAdmin).Get("/metrics", metricsRegistry.Handler)
	}
	if cfg.StorageDriver == "local" {
		router.With(auth.Authenticate(tokenManager)).Handle("/uploads/proofs/*", http.StripPrefix("/uploads/proofs/", http.FileServer(http.Dir(cfg.StorageLocalPath+"/proofs"))))
	}
	if cfg.AppEnv != "production" {
		router.Get("/swagger/openapi.yaml", docs.Spec)
		router.Get("/swagger/index.html", docs.UI)
		router.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/swagger/index.html", http.StatusTemporaryRedirect)
		})
	}
	router.Route("/api/v1", func(r chi.Router) {
		if cfg.RateLimitEnabled {
			r.With(authLimiter.Middleware).Mount("/auth", auth.NewHandler(authService, logger).Routes())
		} else {
			r.Mount("/auth", auth.NewHandler(authService, logger).Routes())
		}
		r.Post("/contributions/{id}/review-token/validate", contributionHandler.ValidateToken)
		r.Get("/contributions/{id}/proof/review", contributionHandler.ReviewProof)
		r.Group(func(protected chi.Router) {
			protected.Use(auth.Authenticate(tokenManager))
			protected.Mount("/users", user.NewHandler(userService, logger).Routes())
			protected.Mount("/contributions", contributionHandler.Routes(auth.RequireAdmin, authLimiter.Middleware))
			protected.Mount("/assistance-requests", assistanceHandler.Routes(auth.RequireAdmin))
			protected.Mount("/dashboard", dashboardHandler.MemberRoutes())
			protected.With(authLimiter.Middleware).Mount("/notifications", notification.NewHandler(notificationService, logger).MemberRoutes())
			protected.Group(func(adminOnly chi.Router) {
				adminOnly.Use(auth.RequireAdmin)
				adminOnly.Mount("/contribution-plans", contributionplan.NewHandler(planService).Routes())
			})
		})
		r.Route("/admin", func(admin chi.Router) {
			admin.Use(auth.Authenticate(tokenManager), auth.RequireAdmin)
			admin.With(authLimiter.Middleware).Mount("/users", user.NewHandler(userService, logger).AdminRoutes())
			admin.Mount("/contributions", contributionHandler.AdminRoutes())
			admin.Mount("/assistance-requests", assistanceHandler.AdminRoutes())
			admin.Mount("/fund", fund.NewHandler(fundRepo, logger).Routes())
			admin.Mount("/audit-logs", audit.NewHandler(auditRepo, logger).Routes())
			admin.Mount("/dashboard", dashboardHandler.AdminRoutes())
			admin.With(authLimiter.Middleware).Mount("/notifications", notification.NewHandler(notificationService, logger).Routes())
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
