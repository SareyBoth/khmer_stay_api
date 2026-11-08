package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPBakongClientCheckByMD5(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/check_transaction_by_md5" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(401)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["md5"] != "qr-md5" {
			t.Errorf("unexpected md5 %q", body["md5"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"responseCode":0,"responseMessage":"Getting transaction successfully.","data":{"hash":"transaction-hash","fromAccountId":"guest@bank","toAccountId":"merchant@bank","currency":"USD","amount":12.5,"description":"KST-001"}}`))
	}))
	defer server.Close()
	transaction, err := NewHTTPBakongClient(server.URL, "test-token").CheckByMD5(context.Background(), "qr-md5")
	if err != nil {
		t.Fatal(err)
	}
	if transaction.Hash != "transaction-hash" || transaction.ToAccountID != "merchant@bank" || transaction.Amount != 12.5 {
		t.Fatalf("unexpected transaction: %#v", transaction)
	}
}

func TestHTTPBakongClientNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"responseCode":1,"responseMessage":"Transaction could not be found.","data":null}`))
	}))
	defer server.Close()
	if _, err := NewHTTPBakongClient(server.URL, "test-token").CheckByMD5(context.Background(), "missing"); err != ErrTransactionNotFound {
		t.Fatalf("got %v", err)
	}
}
