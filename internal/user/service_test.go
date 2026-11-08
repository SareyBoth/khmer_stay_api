package user

import (
	"context"
	"testing"
)

type memoryUsers struct{ rows map[string]User }

func (m *memoryUsers) Create(_ context.Context, u User) (User, error) {
	for _, existing := range m.rows {
		if existing.Email == u.Email {
			return User{}, ErrEmailTaken
		}
	}
	m.rows[u.ID] = u
	return u, nil
}
func (m *memoryUsers) ByEmail(_ context.Context, email string) (User, error) {
	for _, u := range m.rows {
		if u.Email == email {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}
func (m *memoryUsers) ByID(_ context.Context, id string) (User, error) {
	u, ok := m.rows[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}
func (m *memoryUsers) List(context.Context) ([]User, error) {
	result := []User{}
	for _, u := range m.rows {
		result = append(result, u)
	}
	return result, nil
}
func (m *memoryUsers) UpdateStatus(_ context.Context, id, status string) (User, error) {
	u, ok := m.rows[id]
	if !ok {
		return User{}, ErrNotFound
	}
	u.Status = status
	m.rows[id] = u
	return u, nil
}

func TestCreateAndAuthenticate(t *testing.T) {
	repo := &memoryUsers{rows: map[string]User{}}
	service := NewService(repo)
	created, err := service.Create(context.Background(), CreateRequest{Email: "owner@example.com", Password: "strong-password", Name: "Owner", Role: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	if created.PasswordHash == "strong-password" {
		t.Fatal("password was not hashed")
	}
	if created.Status != "Pending" {
		t.Fatalf("got status %s want Pending", created.Status)
	}
	if _, err = service.Authenticate(context.Background(), "owner@example.com", "strong-password"); err == nil {
		t.Fatal("pending account authenticated before email verification")
	}
	if _, err = service.UpdateStatus(context.Background(), created.ID, "Active"); err != nil {
		t.Fatal(err)
	}
	authenticated, err := service.Authenticate(context.Background(), "owner@example.com", "strong-password")
	if err != nil {
		t.Fatal(err)
	}
	if authenticated.ID != created.ID {
		t.Fatalf("got %s want %s", authenticated.ID, created.ID)
	}
}
func TestRejectsPrivilegedSelfRegistration(t *testing.T) {
	service := NewService(&memoryUsers{rows: map[string]User{}})
	if _, err := service.Create(context.Background(), CreateRequest{Email: "bad@example.com", Password: "strong-password", Name: "Bad", Role: "admin"}); err == nil {
		t.Fatal("expected role validation error")
	}
}
