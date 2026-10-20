package postgres

import (
	"context"
	"database/sql"
	"errors"

	"workshop-be/domain"
)

type UserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *UserRepository { return &UserRepository{db: db} }

const userColumns = `id, name, email, password, role, position, ministry, created_at, updated_at`

func scanUser(s interface{ Scan(...any) error }) (*domain.User, error) {
	var u domain.User
	if err := s.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.Position, &u.Ministry, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return u, err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return u, err
}

func (r *UserRepository) ListFunctionaries(ctx context.Context, role *domain.Role) ([]domain.User, error) {
	q := `SELECT ` + userColumns + ` FROM users WHERE role IN ('bph','staff')`
	args := []any{}
	if role != nil {
		q += ` AND role = $1`
		args = append(args, string(*role))
	}
	q += ` ORDER BY name, id`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []domain.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}
