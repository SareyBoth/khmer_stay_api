package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/auth"
	"github.com/khmerstay/gin-api/internal/http/response"
)

const ClaimsKey = "auth.claims"

func Authenticate(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "authentication required")
			return
		}
		claims, err := tokens.Parse(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, role := range roles {
		allowed[role] = true
	}
	return func(c *gin.Context) {
		claims, ok := Current(c)
		if !ok || !allowed[claims.Role] {
			response.Error(c, http.StatusForbidden, "permission denied")
			return
		}
		c.Next()
	}
}

func Current(c *gin.Context) (auth.Claims, bool) {
	value, ok := c.Get(ClaimsKey)
	if !ok {
		return auth.Claims{}, false
	}
	claims, ok := value.(auth.Claims)
	return claims, ok
}
