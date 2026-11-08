package auth

import (
	"context"

	"github.com/khmerstay/gin-api/internal/user"
)

type Service struct {
	users  *user.Service
	tokens *TokenManager
}

func NewService(users *user.Service, tokens *TokenManager) *Service { return &Service{users, tokens} }
func (s *Service) Login(ctx context.Context, email, password string) (string, user.User, error) {
	u, err := s.users.Authenticate(ctx, email, password)
	if err != nil {
		return "", user.User{}, err
	}
	token, err := s.tokens.Sign(u.ID, u.Email, u.Name, u.Role)
	return token, u, err
}

func (s *Service) Register(ctx context.Context, request user.CreateRequest) (user.User, error) {
	return s.users.Create(ctx, request)
}

func (s *Service) TokenForEmail(ctx context.Context, email string) (string, user.User, error) {
	u, err := s.users.ByEmail(ctx, email)
	if err != nil {
		return "", user.User{}, err
	}
	if u.Status != "Active" {
		return "", user.User{}, user.ErrNotFound
	}
	token, err := s.tokens.Sign(u.ID, u.Email, u.Name, u.Role)
	return token, u, err
}
