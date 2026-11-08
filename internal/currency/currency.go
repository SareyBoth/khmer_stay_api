package currency

import (
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/http/response"
)

// DefaultNBCRate is the standard National Bank of Cambodia indicative exchange rate
const DefaultNBCRate = 4100.0

type Converter struct {
	rate float64
}

func NewConverter(rate float64) *Converter {
	if rate <= 0 {
		rate = DefaultNBCRate
	}
	return &Converter{rate: rate}
}

func (c *Converter) Rate() float64 {
	return c.rate
}

func (c *Converter) SetRate(rate float64) {
	if rate > 0 {
		c.rate = rate
	}
}

// USDToKHR converts a USD amount to Khmer Riel, rounding to nearest 100 Riels
func (c *Converter) USDToKHR(usd float64) float64 {
	khr := usd * c.rate
	return math.Round(khr/100) * 100
}

// KHRToUSD converts a Khmer Riel amount to USD, rounding to 2 decimal places
func (c *Converter) KHRToUSD(khr float64) float64 {
	usd := khr / c.rate
	return math.Round(usd*100) / 100
}

type RatesResponse struct {
	BaseCurrency   string    `json:"baseCurrency"`
	TargetCurrency string    `json:"targetCurrency"`
	Rate           float64   `json:"rate"`
	Source         string    `json:"source"`
	Timestamp      time.Time `json:"timestamp"`
}

// Handler returns the current exchange rate
func (c *Converter) Handler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		response.JSON(ctx, http.StatusOK, RatesResponse{
			BaseCurrency:   "USD",
			TargetCurrency: "KHR",
			Rate:           c.rate,
			Source:         "National Bank of Cambodia (NBC) Indicative",
			Timestamp:      time.Now().UTC(),
		})
	}
}
