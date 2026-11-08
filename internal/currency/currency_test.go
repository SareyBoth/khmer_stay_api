package currency

import (
	"testing"
)

func TestUSDToKHRConversion(t *testing.T) {
	c := NewConverter(4100.0)

	tests := []struct {
		usd  float64
		want float64
	}{
		{usd: 1.0, want: 4100.0},
		{usd: 10.0, want: 41000.0},
		{usd: 12.50, want: 51300.0}, // 51250 rounds to 51300
		{usd: 25.0, want: 102500.0},
	}

	for _, tt := range tests {
		got := c.USDToKHR(tt.usd)
		if got != tt.want {
			t.Errorf("USDToKHR(%f) = %f, want %f", tt.usd, got, tt.want)
		}
	}
}

func TestKHRToUSDConversion(t *testing.T) {
	c := NewConverter(4100.0)

	tests := []struct {
		khr  float64
		want float64
	}{
		{khr: 4100.0, want: 1.0},
		{khr: 41000.0, want: 10.0},
		{khr: 51250.0, want: 12.50},
	}

	for _, tt := range tests {
		got := c.KHRToUSD(tt.khr)
		if got != tt.want {
			t.Errorf("KHRToUSD(%f) = %f, want %f", tt.khr, got, tt.want)
		}
	}
}
