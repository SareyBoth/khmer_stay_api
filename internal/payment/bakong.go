package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrTransactionNotFound = errors.New("transaction not found")

type Transaction struct {
	Hash, FromAccountID, ToAccountID, Currency, Description string
	Amount                                                  float64
}
type BakongClient interface {
	CheckByMD5(context.Context, string) (Transaction, error)
}
type HTTPBakongClient struct {
	baseURL, token string
	client         *http.Client
}

func NewHTTPBakongClient(baseURL, token string) *HTTPBakongClient {
	return &HTTPBakongClient{strings.TrimRight(baseURL, "/"), token, &http.Client{Timeout: 10 * time.Second}}
}
func (c *HTTPBakongClient) CheckByMD5(ctx context.Context, md5Hash string) (Transaction, error) {
	body, _ := json.Marshal(map[string]string{"md5": md5Hash})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/check_transaction_by_md5", bytes.NewReader(body))
	if err != nil {
		return Transaction{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return Transaction{}, fmt.Errorf("Bakong request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return Transaction{}, fmt.Errorf("Bakong returned %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var envelope struct {
		ResponseCode    int    `json:"responseCode"`
		ResponseMessage string `json:"responseMessage"`
		Data            *struct {
			Hash          string          `json:"hash"`
			FromAccountID string          `json:"fromAccountId"`
			ToAccountID   string          `json:"toAccountId"`
			Currency      string          `json:"currency"`
			Amount        json.RawMessage `json:"amount"`
			Description   string          `json:"description"`
		} `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&envelope); err != nil {
		return Transaction{}, fmt.Errorf("decode Bakong response: %w", err)
	}
	if envelope.ResponseCode != 0 || envelope.Data == nil {
		return Transaction{}, ErrTransactionNotFound
	}
	amount, err := parseAmount(envelope.Data.Amount)
	if err != nil {
		return Transaction{}, err
	}
	return Transaction{Hash: envelope.Data.Hash, FromAccountID: envelope.Data.FromAccountID, ToAccountID: envelope.Data.ToAccountID, Currency: envelope.Data.Currency, Amount: amount, Description: envelope.Data.Description}, nil
}
func parseAmount(raw json.RawMessage) (float64, error) {
	var number json.Number
	if len(raw) == 0 {
		return 0, errors.New("Bakong response missing amount")
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
		return strconv.ParseFloat(text, 64)
	}
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, err
	}
	return number.Float64()
}
