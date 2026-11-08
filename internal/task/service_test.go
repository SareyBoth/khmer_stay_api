package task

import (
	"context"
	"testing"
)

type memoryTasks struct{ rows map[string]Task }

func (m *memoryTasks) Create(_ context.Context, t Task) (Task, error) {
	m.rows[t.ID] = t
	return t, nil
}
func (m *memoryTasks) List(_ context.Context, userID string) ([]Task, error) {
	result := []Task{}
	for _, t := range m.rows {
		if t.UserID == userID {
			result = append(result, t)
		}
	}
	return result, nil
}
func (m *memoryTasks) ByID(_ context.Context, userID, id string) (Task, error) {
	t, ok := m.rows[id]
	if !ok || t.UserID != userID {
		return Task{}, ErrNotFound
	}
	return t, nil
}
func (m *memoryTasks) Update(_ context.Context, t Task) (Task, error) {
	m.rows[t.ID] = t
	return t, nil
}
func (m *memoryTasks) Delete(_ context.Context, userID, id string) error {
	t, ok := m.rows[id]
	if !ok || t.UserID != userID {
		return ErrNotFound
	}
	delete(m.rows, id)
	return nil
}
func TestTaskOwnership(t *testing.T) {
	repo := &memoryTasks{rows: map[string]Task{}}
	service := NewService(repo)
	created, err := service.Create(context.Background(), "owner-1", CreateRequest{Title: " Prepare stay ", Description: "Open room calendar"})
	if err != nil {
		t.Fatal(err)
	}
	done := true
	updated, err := service.Update(context.Background(), "owner-1", created.ID, UpdateRequest{Completed: &done})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Completed || updated.Title != "Prepare stay" {
		t.Fatalf("unexpected task: %#v", updated)
	}
	if _, err = service.Update(context.Background(), "owner-2", created.ID, UpdateRequest{Completed: &done}); err != ErrNotFound {
		t.Fatalf("expected ownership protection, got %v", err)
	}
}
