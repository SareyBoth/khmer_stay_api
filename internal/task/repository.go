package task

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("task not found")

type Repository interface {
	Create(context.Context, Task) (Task, error)
	List(context.Context, string) ([]Task, error)
	ByID(context.Context, string, string) (Task, error)
	Update(context.Context, Task) (Task, error)
	Delete(context.Context, string, string) error
}
