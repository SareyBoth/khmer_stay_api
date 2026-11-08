package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}
type TokenManager struct {
	secret   []byte
	lifetime time.Duration
}

func NewTokenManager(secret string, lifetime time.Duration) *TokenManager {
	return &TokenManager{[]byte(secret), lifetime}
}
func (m *TokenManager) Sign(userID, email, name, role string) (string, error) {
	now := time.Now()
	claims := Claims{UserID: userID, Email: email, Name: name, Role: role, RegisteredClaims: jwt.RegisteredClaims{ID: uuid.NewString(), Subject: userID, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(m.lifetime)), Issuer: "khmerstay-api"}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}
func (m *TokenManager) Parse(raw string) (Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithIssuer("khmerstay-api"), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return Claims{}, errors.New("invalid token")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return Claims{}, errors.New("invalid claims")
	}
	return *claims, nil
}
