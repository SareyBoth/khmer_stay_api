package auth

import (
	"regexp"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	for range 20 {
		code, err := generateCode()
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^\d{6}$`).MatchString(code) {
			t.Fatalf("invalid OTP %q", code)
		}
	}
}
