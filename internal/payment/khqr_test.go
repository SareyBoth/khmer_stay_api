package payment

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
	"time"
)

func TestGenerateKHQR(t *testing.T) {
	now := time.UnixMilli(1760000000000)
	result, err := GenerateKHQR(KHQRConfig{AccountID: "merchant@bank", MerchantName: "KhmerStay", MerchantCity: "PHNOM PENH", Currency: "USD", TTL: 10 * time.Minute}, 12.50, "KST-001", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Payload) < 50 || result.Payload[len(result.Payload)-8:len(result.Payload)-4] != "6304" {
		t.Fatalf("invalid payload %s", result.Payload)
	}
	sum := md5.Sum([]byte(result.Payload))
	if result.MD5 != hex.EncodeToString(sum[:]) {
		t.Fatal("md5 mismatch")
	}
	if result.ExpiresAt.Sub(now) != 10*time.Minute {
		t.Fatal("incorrect expiration")
	}
}
func TestRejectsLongExpiry(t *testing.T) {
	_, err := GenerateKHQR(KHQRConfig{AccountID: "merchant@bank", MerchantName: "KhmerStay", MerchantCity: "PHNOM PENH", Currency: "USD", TTL: 11 * time.Minute}, 1, "KST-001", time.Now())
	if err == nil {
		t.Fatal("expected expiry validation error")
	}
}
