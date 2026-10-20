package postgres

import (
	"context"
	"database/sql"
	"errors"

	"workshop-be/domain"
)

type AspectRepository struct{ db *sql.DB }

func NewAspectRepository(db *sql.DB) *AspectRepository { return &AspectRepository{db: db} }

const aspectColumns = `id, name, description, role_type, created_at, updated_at`

func scanAspect(s interface{ Scan(...any) error }) (*domain.Aspect, error) {
	var a domain.Aspect
	if err := s.Scan(&a.ID, &a.Name, &a.Description, &a.RoleType, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AspectRepository) List(ctx context.Context, roleType *domain.RoleType) ([]domain.Aspect, error) {
	q := `SELECT ` + aspectColumns + ` FROM assessment_aspects`
	args := []any{}
	if roleType != nil {
		q += ` WHERE role_type = $1`
		args = append(args, string(*roleType))
	}
	q += ` ORDER BY role_type, id`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Aspect{}
	for rows.Next() {
		a, err := scanAspect(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *AspectRepository) GetByID(ctx context.Context, id int64) (*domain.Aspect, error) {
	a, err := scanAspect(r.db.QueryRowContext(ctx, `SELECT `+aspectColumns+` FROM assessment_aspects WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return a, err
}

func (r *AspectRepository) Create(ctx context.Context, a *domain.Aspect) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO assessment_aspects (name, description, role_type) VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		a.Name, a.Description, string(a.RoleType)).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	if code, _ := pgCode(err); code == codeUniqueViolation {
		return domain.ErrAspectDuplicate
	}
	return err
}

func (r *AspectRepository) Update(ctx context.Context, a *domain.Aspect) error {
	err := r.db.QueryRowContext(ctx,
		`UPDATE assessment_aspects SET name = $1, description = $2, role_type = $3, updated_at = now()
		 WHERE id = $4 RETURNING updated_at`,
		a.Name, a.Description, string(a.RoleType), a.ID).Scan(&a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if code, _ := pgCode(err); code == codeUniqueViolation {
		return domain.ErrAspectDuplicate
	}
	return err
}

func (r *AspectRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM assessment_aspects WHERE id = $1`, id)
	if code, _ := pgCode(err); code == codeForeignKey {
		return domain.ErrAspectInUse // dipakai bersamaan oleh raport baru
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AspectRepository) IsUsed(ctx context.Context, id int64) (bool, error) {
	var used bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM report_scores WHERE aspect_id = $1)`, id).Scan(&used)
	return used, err
}
