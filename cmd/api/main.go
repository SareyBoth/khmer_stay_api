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

	"github.com/khmerstay/gin-api/internal/auth"
	"github.com/khmerstay/gin-api/internal/config"
	"github.com/khmerstay/gin-api/internal/currency"
	"github.com/khmerstay/gin-api/internal/database"
	"github.com/khmerstay/gin-api/internal/events"
	httpapi "github.com/khmerstay/gin-api/internal/http"
	"github.com/khmerstay/gin-api/internal/http/middleware"
	"github.com/khmerstay/gin-api/internal/payment"
	"github.com/khmerstay/gin-api/internal/platform"
	"github.com/khmerstay/gin-api/internal/storage"
	"github.com/khmerstay/gin-api/internal/task"
	"github.com/khmerstay/gin-api/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db, cfg.MigrationsDir); err != nil {
		log.Fatal(err)
	}
	platformRepo := platform.NewRepository(db)
	if cfg.SeedDemo {
		if err = platformRepo.SeedDemo(ctx); err != nil {
			log.Fatal(err)
		}
	}
	userRepo := user.NewPostgresRepository(db)
	userService := user.NewService(userRepo)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTLifetime)
	refreshStore := auth.NewRefreshTokenStore(db, 30*24*time.Hour)
	broker := events.NewBroker()
	converter := currency.NewConverter(cfg.NBCExchangeRate)
	limiter := middleware.NewRateLimiter(cfg.RateLimitRequests, time.Minute)

	var uploader storage.Uploader
	if cfg.StorageDriver == "s3" {
		uploader = storage.NewS3Storage(storage.S3Config{
			Endpoint:        cfg.S3.Endpoint,
			Bucket:          cfg.S3.Bucket,
			AccessKeyID:     cfg.S3.AccessKeyID,
			SecretAccessKey: cfg.S3.SecretAccessKey,
			Region:          cfg.S3.Region,
			PublicURL:       cfg.S3.PublicURL,
		}, storage.NewLocalStorage(cfg.UploadsDir, "/uploads"))
	} else {
		uploader = storage.NewLocalStorage(cfg.UploadsDir, "/uploads")
	}

	paymentRepo := payment.NewRepository(db)
	bakongClient := payment.NewHTTPBakongClient(cfg.Bakong.APIBaseURL, cfg.Bakong.Token)
	paymentConfig := payment.Config{
		KHQR: payment.KHQRConfig{
			AccountID:    cfg.Bakong.AccountID,
			MerchantName: cfg.Bakong.Merchant,
			MerchantCity: cfg.Bakong.City,
			Currency:     cfg.Bakong.Currency,
			TTL:          cfg.Bakong.QRTimeout,
		},
		TokenConfigured: cfg.Bakong.Token != "",
		WebhookSecret:   cfg.Bakong.WebhookSecret,
	}
	paymentService := payment.NewService(paymentRepo, bakongClient, paymentConfig)
	paymentService.OnPaid = func(b payment.Booking) {
		broker.Publish(b.TravelerID, events.Event{Type: "payment_received", Data: b})
		broker.Publish(b.OwnerID, events.Event{Type: "payment_received", Data: b})
	}

	// Start background booking reaper for unpaid booking auto-expiry
	reaperCtx, cancelReaper := context.WithCancel(context.Background())
	defer cancelReaper()
	platform.StartBookingReaper(reaperCtx, db, 15*time.Minute, time.Minute, broker)

	var mailer auth.Mailer
	if cfg.Resend.APIKey != "" {
		mailer = auth.ResendMailer{APIKey: cfg.Resend.APIKey, From: cfg.Resend.From, BaseURL: cfg.Resend.APIBaseURL}
	} else if cfg.SMTP.Host != "" {
		mailer = auth.SMTPMailer{Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, User: cfg.SMTP.User, Password: cfg.SMTP.Password, From: cfg.SMTP.From}
	} else if cfg.AppEnv != "production" {
		mailer = auth.DevelopmentMailer{}
	} else {
		log.Fatal("configure RESEND_API_KEY or SMTP_HOST for production email delivery")
	}
	otpService := auth.NewOTPService(db, mailer)
	googleService := auth.NewGoogleService(auth.GoogleIDTokenVerifier{Audience: cfg.Google.ClientID}, auth.NewPostgresGoogleAccountStore(db), tokens)
	router := httpapi.NewRouter(httpapi.Dependencies{
		Config:    cfg,
		Tokens:    tokens,
		Auth:      auth.NewHandler(auth.NewService(userService, tokens), otpService, googleService, refreshStore),
		Users:     user.NewHandler(userService),
		Tasks:     task.NewHandler(task.NewService(task.NewPostgresRepository(db))),
		Platform:  platform.NewHandler(platformRepo, cfg.UploadsDir, uploader, broker),
		Payments:  payment.NewHandler(paymentService, cfg.Bakong.AccountID != "", cfg.Bakong.Token != ""),
		Broker:    broker,
		Converter: converter,
		Limiter:   limiter,
	})
	server := &http.Server{Addr: ":" + cfg.Port, Handler: router, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("KhmerStay API listening on :%s", cfg.Port)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("server: %v", serveErr)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	log.Printf("received signal: %v, shutting down...", sig)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
