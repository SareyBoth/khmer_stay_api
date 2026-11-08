package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResendMailerSendsAuthenticatedEmail(t *testing.T) {
	var authorization string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		if r.URL.Path != "/emails" {
			t.Fatalf("path = %s, want /emails", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	mailer := ResendMailer{APIKey: "re_test", From: "KhmerStay <hello@example.com>", BaseURL: server.URL, Client: server.Client()}
	if err := mailer.Send(context.Background(), "guest@example.com", "Verify", "Code 123456"); err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer re_test" {
		t.Fatalf("authorization = %q", authorization)
	}
	if payload["from"] != "KhmerStay <hello@example.com>" || payload["subject"] != "Verify" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestResendMailerReturnsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"invalid sender"}`, http.StatusUnprocessableEntity)
	}))
	defer server.Close()
	mailer := ResendMailer{APIKey: "re_test", From: "bad@example.com", BaseURL: server.URL, Client: server.Client()}
	if err := mailer.Send(context.Background(), "guest@example.com", "Verify", "Code"); err == nil {
		t.Fatal("expected provider error")
	}
}
