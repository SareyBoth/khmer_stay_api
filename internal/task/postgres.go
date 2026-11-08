package task

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db} }
func scan(row pgx.Row) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Completed, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}
func (r *PostgresRepository) Create(ctx context.Context, t Task) (Task, error) {
	return scan(r.db.QueryRow(ctx, `INSERT INTO tasks(id,user_id,title,description,completed)VALUES($1,$2,$3,$4,$5)RETURNING id,user_id,title,description,completed,created_at,updated_at`, t.ID, t.UserID, t.Title, t.Description, t.Completed))
}
func (r *PostgresRepository) List(ctx context.Context, userID string) ([]Task, error) {
	rows, err := r.db.Query(ctx, `SELECT id,user_id,title,description,completed,created_at,updated_at FROM tasks WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Task{}
	for rows.Next() {
		t, e := scan(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
func (r *PostgresRepository) ByID(ctx context.Context, userID, id string) (Task, error) {
	return scan(r.db.QueryRow(ctx, `SELECT id,user_id,title,description,completed,created_at,updated_at FROM tasks WHERE id=$1 AND user_id=$2`, id, userID))
}
func (r *PostgresRepository) Update(ctx context.Context, t Task) (Task, error) {
	return scan(r.db.QueryRow(ctx, `UPDATE tasks SET title=$3,description=$4,completed=$5,updated_at=NOW() WHERE id=$1 AND user_id=$2 RETURNING id,user_id,title,description,completed,created_at,updated_at`, t.ID, t.UserID, t.Title, t.Description, t.Completed))
}
func (r *PostgresRepository) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM tasks WHERE id=$1 AND user_id=$2`, id, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
