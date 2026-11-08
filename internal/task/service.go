package task

import (
	"context"
	"github.com/google/uuid"
	"strings"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo} }
func (s *Service) Create(ctx context.Context, userID string, req CreateRequest) (Task, error) {
	return s.repo.Create(ctx, Task{ID: uuid.NewString(), UserID: userID, Title: strings.TrimSpace(req.Title), Description: strings.TrimSpace(req.Description)})
}
func (s *Service) List(ctx context.Context, userID string) ([]Task, error) {
	return s.repo.List(ctx, userID)
}
func (s *Service) Update(ctx context.Context, userID, id string, req UpdateRequest) (Task, error) {
	t, err := s.repo.ByID(ctx, userID, id)
	if err != nil {
		return Task{}, err
	}
	if req.Title != nil {
		t.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		t.Description = strings.TrimSpace(*req.Description)
	}
	if req.Completed != nil {
		t.Completed = *req.Completed
	}
	return s.repo.Update(ctx, t)
}
func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}
