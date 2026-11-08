package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterBlocksExcessRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	limiter := NewRateLimiter(2, time.Minute)

	r.GET("/test", limiter.Middleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Request 1: Allowed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("req 1: got %d, want 200", w1.Code)
	}

	// Request 2: Allowed
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("req 2: got %d, want 200", w2.Code)
	}

	// Request 3: Rate limited (exceeded capacity 2)
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("req 3: got %d, want 429", w3.Code)
	}
}
