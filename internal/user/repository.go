package user

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("user not found")
var ErrEmailTaken = errors.New("email already registered")

type Repository interface {
	Create(context.Context, User) (User, error)
	ByEmail(context.Context, string) (User, error)
	ByID(context.Context, string) (User, error)
	List(context.Context) ([]User, error)
	UpdateStatus(context.Context, string, string) (User, error)
}
