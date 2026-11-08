package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khmerstay/gin-api/internal/user"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

var ErrGoogleNotConfigured = errors.New("Google sign-in is not configured")
var ErrInvalidGoogleToken = errors.New("invalid Google identity token")
var ErrGoogleEmailConflict = errors.New("this email already uses email sign-in; sign in with email first")
var ErrGoogleAccountDisabled = errors.New("this account is not active")

type GoogleIdentity struct {
	Subject       string
	Email         string
	Name          string
	Picture       string
	HostedDomain  string
	EmailVerified bool
}

func (i GoogleIdentity) authoritativeEmail() bool {
	email := strings.ToLower(i.Email)
	return strings.HasSuffix(email, "@gmail.com") || strings.TrimSpace(i.HostedDomain) != ""
}

type GoogleVerifier interface {
	Verify(context.Context, string) (GoogleIdentity, error)
}

type GoogleIDTokenVerifier struct{ Audience string }

func (v GoogleIDTokenVerifier) Verify(ctx context.Context, rawToken string) (GoogleIdentity, error) {
	if strings.TrimSpace(v.Audience) == "" {
		return GoogleIdentity{}, ErrGoogleNotConfigured
	}
	payload, err := idtoken.Validate(ctx, strings.TrimSpace(rawToken), v.Audience)
	if err != nil {
		return GoogleIdentity{}, ErrInvalidGoogleToken
	}
	identity := GoogleIdentity{
		Subject:       payload.Subject,
		Email:         claimString(payload.Claims, "email"),
		Name:          claimString(payload.Claims, "name"),
		Picture:       claimString(payload.Claims, "picture"),
		HostedDomain:  claimString(payload.Claims, "hd"),
		EmailVerified: claimBool(payload.Claims, "email_verified"),
	}
	if identity.Subject == "" || identity.Email == "" || !identity.EmailVerified {
		return GoogleIdentity{}, ErrInvalidGoogleToken
	}
	if strings.TrimSpace(identity.Name) == "" {
		identity.Name = strings.Split(identity.Email, "@")[0]
	}
	return identity, nil
}

func claimString(claims map[string]any, key string) string {
	value, _ := claims[key].(string)
	return strings.TrimSpace(value)
}

func claimBool(claims map[string]any, key string) bool {
	value, ok := claims[key].(bool)
	return ok && value
}

type GoogleAccountStore interface {
	FindOrCreate(context.Context, GoogleIdentity) (user.User, error)
}

type PostgresGoogleAccountStore struct{ db *pgxpool.Pool }

func NewPostgresGoogleAccountStore(db *pgxpool.Pool) *PostgresGoogleAccountStore {
	return &PostgresGoogleAccountStore{db: db}
}

func scanGoogleUser(row pgx.Row) (user.User, error) {
	var account user.User
	err := row.Scan(&account.ID, &account.Email, &account.PasswordHash, &account.Name, &account.Role, &account.Status, &account.CreatedAt)
	return account, err
}

func (s *PostgresGoogleAccountStore) FindOrCreate(ctx context.Context, identity GoogleIdentity) (user.User, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return user.User{}, err
	}
	defer tx.Rollback(ctx)
	account, err := scanGoogleUser(tx.QueryRow(ctx, `
		SELECT u.id,u.email,u.password_hash,u.name,u.role,u.status,u.created_at
		FROM user_identities i JOIN users u ON u.id=i.user_id
		WHERE i.provider='google' AND i.subject=$1 FOR UPDATE OF i,u`, identity.Subject))
	if err == nil {
		if account.Status != "Active" {
			return user.User{}, ErrGoogleAccountDisabled
		}
		_, err = tx.Exec(ctx, `UPDATE user_identities SET last_login_at=NOW(),provider_email=LOWER($2),profile=$3 WHERE provider='google' AND subject=$1`, identity.Subject, identity.Email, googleProfile(identity))
		if err != nil {
			return user.User{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return user.User{}, err
		}
		return account, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return user.User{}, err
	}

	account, err = scanGoogleUser(tx.QueryRow(ctx, `SELECT id,email,password_hash,name,role,status,created_at FROM users WHERE email=LOWER($1) FOR UPDATE`, identity.Email))
	if err == nil {
		if !identity.authoritativeEmail() {
			return user.User{}, ErrGoogleEmailConflict
		}
		if account.Status == "Suspended" {
			return user.User{}, ErrGoogleAccountDisabled
		}
		if account.Status == "Pending" {
			account, err = scanGoogleUser(tx.QueryRow(ctx, `UPDATE users SET status='Active' WHERE id=$1 RETURNING id,email,password_hash,name,role,status,created_at`, account.ID))
			if err != nil {
				return user.User{}, err
			}
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		passwordHash, hashErr := bcrypt.GenerateFromPassword([]byte(uuid.NewString()+uuid.NewString()), bcrypt.DefaultCost)
		if hashErr != nil {
			return user.User{}, hashErr
		}
		account, err = scanGoogleUser(tx.QueryRow(ctx, `INSERT INTO users(id,email,password_hash,name,role,status) VALUES($1,LOWER($2),$3,$4,'traveler','Active') RETURNING id,email,password_hash,name,role,status,created_at`, uuid.NewString(), identity.Email, string(passwordHash), identity.Name))
		if err != nil {
			return user.User{}, err
		}
	} else {
		return user.User{}, err
	}

	_, err = tx.Exec(ctx, `INSERT INTO user_identities(provider,subject,user_id,provider_email,profile) VALUES('google',$1,$2,LOWER($3),$4)`, identity.Subject, account.ID, identity.Email, googleProfile(identity))
	if err != nil {
		return user.User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return user.User{}, err
	}
	return account, nil
}

func googleProfile(identity GoogleIdentity) []byte {
	raw, _ := json.Marshal(map[string]any{"name": identity.Name, "picture": identity.Picture, "hostedDomain": identity.HostedDomain})
	return raw
}

type GoogleService struct {
	verifier GoogleVerifier
	accounts GoogleAccountStore
	tokens   *TokenManager
}

func NewGoogleService(verifier GoogleVerifier, accounts GoogleAccountStore, tokens *TokenManager) *GoogleService {
	return &GoogleService{verifier: verifier, accounts: accounts, tokens: tokens}
}

func (s *GoogleService) Authenticate(ctx context.Context, rawToken string) (string, user.User, error) {
	identity, err := s.verifier.Verify(ctx, rawToken)
	if err != nil {
		return "", user.User{}, err
	}
	account, err := s.accounts.FindOrCreate(ctx, identity)
	if err != nil {
		return "", user.User{}, err
	}
	token, err := s.tokens.Sign(account.ID, account.Email, account.Name, account.Role)
	return token, account, err
}
