package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Bakong struct {
	AccountID     string
	Token         string
	APIBaseURL    string
	Merchant      string
	City          string
	Currency      string
	QRTimeout     time.Duration
	WebhookSecret string
}

type S3 struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	PublicURL       string
}

type SMTP struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

type Resend struct {
	APIKey     string
	From       string
	APIBaseURL string
}

type GoogleOAuth struct {
	ClientID string
}

type Config struct {
	AppEnv            string
	Port              string
	DatabaseURL       string
	JWTSecret         string
	JWTLifetime       time.Duration
	AllowedOrigins    []string
	MigrationsDir     string
	UploadsDir        string
	SeedDemo          bool
	Bakong            Bakong
	SMTP              SMTP
	Resend            Resend
	Google            GoogleOAuth
	StorageDriver     string
	S3                S3
	RateLimitRequests int
	NBCExchangeRate   float64
}

func Load() (Config, error) {
	_ = godotenv.Load()
	jwtMinutes, err := intEnv("JWT_LIFETIME_MINUTES", 720)
	if err != nil {
		return Config{}, err
	}
	qrSeconds, err := intEnv("BAKONG_QR_TTL_SECONDS", 600)
	if err != nil {
		return Config{}, err
	}
	if qrSeconds < 60 || qrSeconds > 600 {
		return Config{}, errors.New("BAKONG_QR_TTL_SECONDS must be between 60 and 600")
	}
	rateLimit, _ := intEnv("RATE_LIMIT_PER_MINUTE", 60)
	exchangeRate := 4100.0
	if rateStr := os.Getenv("NBC_EXCHANGE_RATE"); rateStr != "" {
		if r, err := strconv.ParseFloat(rateStr, 64); err == nil && r > 0 {
			exchangeRate = r
		}
	}
	cfg := Config{
		AppEnv: env("APP_ENV", "development"), Port: env("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"), JWTSecret: os.Getenv("JWT_SECRET"),
		JWTLifetime:    time.Duration(jwtMinutes) * time.Minute,
		AllowedOrigins: splitCSV(env("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")),
		MigrationsDir:  env("MIGRATIONS_DIR", "migrations"), UploadsDir: env("UPLOADS_DIR", "uploads"),
		SeedDemo: boolEnv("SEED_DEMO", true),
		Bakong:   Bakong{AccountID: os.Getenv("BAKONG_ACCOUNT_ID"), Token: os.Getenv("BAKONG_API_TOKEN"), APIBaseURL: env("BAKONG_API_BASE_URL", "https://api-bakong.nbc.gov.kh"), Merchant: env("BAKONG_MERCHANT_NAME", "KhmerStay"), City: env("BAKONG_MERCHANT_CITY", "PHNOM PENH"), Currency: strings.ToUpper(env("BAKONG_CURRENCY", "USD")), QRTimeout: time.Duration(qrSeconds) * time.Second, WebhookSecret: os.Getenv("BAKONG_WEBHOOK_SECRET")},
		SMTP:     SMTP{Host: os.Getenv("SMTP_HOST"), Port: env("SMTP_PORT", "587"), User: os.Getenv("SMTP_USER"), Password: os.Getenv("SMTP_PASSWORD"), From: env("SMTP_FROM", "no-reply@khmerstay.local")},
		Resend:   Resend{APIKey: os.Getenv("RESEND_API_KEY"), From: env("RESEND_FROM", "KhmerStay <no-reply@khmerstay.com>"), APIBaseURL: env("RESEND_API_BASE_URL", "https://api.resend.com")},
		Google:   GoogleOAuth{ClientID: os.Getenv("GOOGLE_CLIENT_ID")},
		StorageDriver: env("STORAGE_DRIVER", "local"),
		S3: S3{
			Endpoint:        os.Getenv("S3_ENDPOINT"),
			Bucket:          os.Getenv("S3_BUCKET"),
			AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
			Region:          env("S3_REGION", "ap-southeast-1"),
			PublicURL:       os.Getenv("S3_PUBLIC_URL"),
		},
		RateLimitRequests: rateLimit,
		NBCExchangeRate:   exchangeRate,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be at least 32 characters")
	}
	if cfg.Bakong.Currency != "USD" && cfg.Bakong.Currency != "KHR" {
		return Config{}, errors.New("BAKONG_CURRENCY must be USD or KHR")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
func splitCSV(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
func intEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}
