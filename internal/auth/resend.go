package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"
)

type ResendMailer struct {
	APIKey  string
	From    string
	BaseURL string
	Client  *http.Client
}

func (m ResendMailer) Send(ctx context.Context, to, subject, body string) error {
	if strings.TrimSpace(m.APIKey) == "" {
		return errors.New("RESEND_API_KEY is not configured")
	}
	if strings.TrimSpace(m.From) == "" {
		return errors.New("RESEND_FROM is not configured")
	}
	baseURL := strings.TrimRight(m.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.resend.com"
	}
	payload := map[string]any{
		"from":    m.From,
		"to":      []string{to},
		"subject": subject,
		"text":    body,
		"html":    `<div style="font-family:Arial,sans-serif;line-height:1.6;color:#2b2118"><h2>KhmerStay</h2><p>` + html.EscapeString(body) + `</p><p style="color:#756d66;font-size:13px">If you did not request this code, you can safely ignore this email.</p></div>`,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/emails", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("resend returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	return nil
}
