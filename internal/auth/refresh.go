package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")

type RefreshTokenStore struct {
	db  *pgxpool.Pool
	ttl time.Duration
}

func NewRefreshTokenStore(db *pgxpool.Pool, ttl time.Duration) *RefreshTokenStore {
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	return &RefreshTokenStore{db: db, ttl: ttl}
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateRandomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *RefreshTokenStore) Create(ctx context.Context, userID string) (string, error) {
	raw, err := generateRandomToken()
	if err != nil {
		return "", err
	}
	hash := hashToken(raw)
	expiresAt := time.Now().Add(s.ttl)
	_, err = s.db.Exec(ctx, `INSERT INTO refresh_tokens(id, user_id, token_hash, expires_at) VALUES($1, $2, $3, $4)`,
		uuid.New(), userID, hash, expiresAt)
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (s *RefreshTokenStore) Rotate(ctx context.Context, rawToken string) (newRawToken string, userID string, err error) {
	if rawToken == "" {
		return "", "", ErrInvalidRefreshToken
	}
	hash := hashToken(rawToken)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	var expiresAt time.Time
	var revoked bool

	err = tx.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked 
		FROM refresh_tokens 
		WHERE token_hash=$1 FOR UPDATE`, hash).Scan(&id, &userID, &expiresAt, &revoked)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrInvalidRefreshToken
		}
		return "", "", err
	}

	if revoked || time.Now().After(expiresAt) {
		return "", "", ErrInvalidRefreshToken
	}

	// Revoke old token
	if _, err = tx.Exec(ctx, `UPDATE refresh_tokens SET revoked=TRUE WHERE id=$1`, id); err != nil {
		return "", "", err
	}

	// Issue new token (rotation)
	newRawToken, err = generateRandomToken()
	if err != nil {
		return "", "", err
	}
	newHash := hashToken(newRawToken)
	newExpiresAt := time.Now().Add(s.ttl)

	if _, err = tx.Exec(ctx, `INSERT INTO refresh_tokens(id, user_id, token_hash, expires_at) VALUES($1, $2, $3, $4)`,
		uuid.New(), userID, newHash, newExpiresAt); err != nil {
		return "", "", err
	}

	return newRawToken, userID, tx.Commit(ctx)
}

func (s *RefreshTokenStore) Revoke(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	hash := hashToken(rawToken)
	_, err := s.db.Exec(ctx, `UPDATE refresh_tokens SET revoked=TRUE WHERE token_hash=$1`, hash)
	return err
}

func (s *RefreshTokenStore) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx, `UPDATE refresh_tokens SET revoked=TRUE WHERE user_id=$1`, userID)
	return err
}
