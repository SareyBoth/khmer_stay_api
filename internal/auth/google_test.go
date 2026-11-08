package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/khmerstay/gin-api/internal/user"
)

type fakeGoogleVerifier struct {
	identity GoogleIdentity
	err      error
}

func (f fakeGoogleVerifier) Verify(context.Context, string) (GoogleIdentity, error) {
	return f.identity, f.err
}

type fakeGoogleAccounts struct {
	account user.User
	err     error
}

func (f fakeGoogleAccounts) FindOrCreate(context.Context, GoogleIdentity) (user.User, error) {
	return f.account, f.err
}

func TestGoogleServiceCreatesKhmerStaySession(t *testing.T) {
	account := user.User{ID: "user-1", Email: "guest@gmail.com", Name: "Guest", Role: "traveler", Status: "Active"}
	service := NewGoogleService(
		fakeGoogleVerifier{identity: GoogleIdentity{Subject: "google-1", Email: account.Email, EmailVerified: true}},
		fakeGoogleAccounts{account: account},
		NewTokenManager("a-test-secret-that-is-at-least-32-characters", time.Hour),
	)
	token, got, err := service.Authenticate(context.Background(), "google-id-token")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || got.Email != account.Email || got.Role != "traveler" {
		t.Fatalf("unexpected authentication result: token=%t user=%+v", token != "", got)
	}
}

func TestGoogleServiceRejectsInvalidToken(t *testing.T) {
	service := NewGoogleService(
		fakeGoogleVerifier{err: ErrInvalidGoogleToken},
		fakeGoogleAccounts{},
		NewTokenManager("a-test-secret-that-is-at-least-32-characters", time.Hour),
	)
	if _, _, err := service.Authenticate(context.Background(), "bad"); !errors.Is(err, ErrInvalidGoogleToken) {
		t.Fatalf("error = %v, want ErrInvalidGoogleToken", err)
	}
}
