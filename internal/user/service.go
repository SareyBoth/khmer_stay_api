package user

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo} }
func (s *Service) Create(ctx context.Context, request CreateRequest) (User, error) {
	role := strings.ToLower(strings.TrimSpace(request.Role))
	if role == "" {
		role = "traveler"
	}
	if role != "traveler" && role != "owner" {
		return User{}, errors.New("role must be traveler or owner")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	return s.repo.Create(ctx, User{ID: uuid.NewString(), Email: strings.TrimSpace(request.Email), PasswordHash: string(hash), Name: strings.TrimSpace(request.Name), Role: role, Status: "Pending"})
}
func (s *Service) Authenticate(ctx context.Context, email, password string) (User, error) {
	u, err := s.repo.ByEmail(ctx, email)
	if err != nil {
		return User{}, errors.New("invalid email or password")
	}
	if u.Status != "Active" {
		return User{}, errors.New("account is not active")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return User{}, errors.New("invalid email or password")
	}
	return u, nil
}
func (s *Service) ByID(ctx context.Context, id string) (User, error) { return s.repo.ByID(ctx, id) }
func (s *Service) ByEmail(ctx context.Context, email string) (User, error) {
	return s.repo.ByEmail(ctx, email)
}
func (s *Service) List(ctx context.Context) ([]User, error) { return s.repo.List(ctx) }
func (s *Service) UpdateStatus(ctx context.Context, id, status string) (User, error) {
	return s.repo.UpdateStatus(ctx, id, status)
}
