package user

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db} }
func scan(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}
func (r *PostgresRepository) Create(ctx context.Context, u User) (User, error) {
	result, err := scan(r.db.QueryRow(ctx, `INSERT INTO users(id,email,password_hash,name,role,status) VALUES($1,LOWER($2),$3,$4,$5,$6) RETURNING id,email,password_hash,name,role,status,created_at`, u.ID, u.Email, u.PasswordHash, u.Name, u.Role, u.Status))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return User{}, ErrEmailTaken
	}
	return result, err
}
func (r *PostgresRepository) ByEmail(ctx context.Context, email string) (User, error) {
	return scan(r.db.QueryRow(ctx, `SELECT id,email,password_hash,name,role,status,created_at FROM users WHERE email=LOWER($1)`, strings.TrimSpace(email)))
}
func (r *PostgresRepository) ByID(ctx context.Context, id string) (User, error) {
	return scan(r.db.QueryRow(ctx, `SELECT id,email,password_hash,name,role,status,created_at FROM users WHERE id=$1`, id))
}
func (r *PostgresRepository) List(ctx context.Context) ([]User, error) {
	rows, err := r.db.Query(ctx, `SELECT id,email,password_hash,name,role,status,created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []User{}
	for rows.Next() {
		u, scanErr := scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, u)
	}
	return result, rows.Err()
}
func (r *PostgresRepository) UpdateStatus(ctx context.Context, id, status string) (User, error) {
	return scan(r.db.QueryRow(ctx, `UPDATE users SET status=$2 WHERE id=$1 RETURNING id,email,password_hash,name,role,status,created_at`, id, status))
}
