package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var ErrOTPRateLimit = errors.New("otp rate limit")
var ErrInvalidOTP = errors.New("invalid or expired otp")

type Mailer interface {
	Send(context.Context, string, string, string) error
}

type SMTPMailer struct{ Host, Port, User, Password, From string }

func (m SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	if m.Host == "" {
		return errors.New("SMTP_HOST is not configured")
	}
	var auth smtp.Auth
	if m.User != "" {
		auth = smtp.PlainAuth("", m.User, m.Password, m.Host)
	}
	message := []byte("From: " + m.From + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	return smtp.SendMail(m.Host+":"+m.Port, auth, m.From, []string{to}, message)
}

type DevelopmentMailer struct{}

func (DevelopmentMailer) Send(_ context.Context, to, subject, body string) error {
	log.Printf("development mail to=%s subject=%q body=%q", to, subject, body)
	return nil
}

type OTPService struct {
	db     *pgxpool.Pool
	mailer Mailer
	now    func() time.Time
}

func NewOTPService(db *pgxpool.Pool, mailer Mailer) *OTPService {
	return &OTPService{db: db, mailer: mailer, now: time.Now}
}

func generateCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func (s *OTPService) RequestReset(ctx context.Context, email string) error {
	return s.Request(ctx, email, "reset")
}

func (s *OTPService) Request(ctx context.Context, email, purpose string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if purpose != "reset" && purpose != "login" && purpose != "verify_email" {
		return ErrInvalidOTP
	}
	var status string
	if err := s.db.QueryRow(ctx, `SELECT status FROM users WHERE LOWER(email)=$1`, email).Scan(&status); err != nil {
		return err
	}
	if (purpose == "verify_email" && status != "Pending") || (purpose != "verify_email" && status != "Active") {
		return ErrInvalidOTP
	}
	var last time.Time
	err := s.db.QueryRow(ctx, `SELECT created_at FROM otp_codes WHERE email=$1 AND purpose=$2 ORDER BY created_at DESC LIMIT 1`, email, purpose).Scan(&last)
	if err == nil && s.now().Sub(last) < time.Minute {
		return ErrOTPRateLimit
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	code, err := generateCode()
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	codeID := uuid.New()
	if _, err = s.db.Exec(ctx, `INSERT INTO otp_codes(id,email,purpose,code_hash,expires_at) VALUES($1,$2,$3,$4,$5)`, codeID, email, purpose, string(hash), s.now().Add(10*time.Minute)); err != nil {
		return err
	}
	subject := "KhmerStay sign-in code"
	body := "Your KhmerStay sign-in code is " + code + ". It expires in 10 minutes."
	if purpose == "reset" {
		subject = "KhmerStay password reset code"
		body = "Your KhmerStay reset code is " + code + ". It expires in 10 minutes."
	} else if purpose == "verify_email" {
		subject = "Verify your KhmerStay email"
		body = "Your KhmerStay verification code is " + code + ". It expires in 10 minutes."
	}
	if err = s.mailer.Send(ctx, email, subject, body); err != nil {
		log.Printf("send email code: %v", err)
		_, _ = s.db.Exec(ctx, `DELETE FROM otp_codes WHERE id=$1`, codeID)
		return err
	}
	return nil
}

func (s *OTPService) Verify(ctx context.Context, email, purpose, code string) error {
	if purpose != "login" && purpose != "verify_email" {
		return ErrInvalidOTP
	}
	email = strings.ToLower(strings.TrimSpace(email))
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id uuid.UUID
	var hash string
	var expires time.Time
	var attempts int
	var used bool
	err = tx.QueryRow(ctx, `SELECT id,code_hash,expires_at,attempts,used FROM otp_codes WHERE email=$1 AND purpose=$2 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, email, purpose).Scan(&id, &hash, &expires, &attempts, &used)
	if err != nil || used || attempts >= 5 || !expires.After(s.now()) {
		return ErrInvalidOTP
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) != nil {
		_, _ = tx.Exec(ctx, `UPDATE otp_codes SET attempts=attempts+1 WHERE id=$1`, id)
		_ = tx.Commit(ctx)
		return ErrInvalidOTP
	}
	if purpose == "verify_email" {
		tag, updateErr := tx.Exec(ctx, `UPDATE users SET status='Active' WHERE LOWER(email)=$1 AND status='Pending'`, email)
		if updateErr != nil || tag.RowsAffected() == 0 {
			return ErrInvalidOTP
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE otp_codes SET used=TRUE WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *OTPService) ResetPassword(ctx context.Context, email, code, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id uuid.UUID
	var hash string
	var expires time.Time
	var attempts int
	var used bool
	err = tx.QueryRow(ctx, `SELECT id,code_hash,expires_at,attempts,used FROM otp_codes WHERE email=$1 AND purpose='reset' ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, email).Scan(&id, &hash, &expires, &attempts, &used)
	if err != nil || used || attempts >= 5 || !expires.After(s.now()) {
		return ErrInvalidOTP
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) != nil {
		_, _ = tx.Exec(ctx, `UPDATE otp_codes SET attempts=attempts+1 WHERE id=$1`, id)
		_ = tx.Commit(ctx)
		return ErrInvalidOTP
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE LOWER(email)=$1`, email, string(passwordHash))
	if err != nil || tag.RowsAffected() == 0 {
		return ErrInvalidOTP
	}
	if _, err = tx.Exec(ctx, `UPDATE otp_codes SET used=TRUE WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
