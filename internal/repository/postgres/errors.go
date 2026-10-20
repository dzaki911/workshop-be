package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	codeUniqueViolation = "23505"
	codeForeignKey      = "23503"
)

// pgCode mengembalikan SQLSTATE dan nama constraint dari error PostgreSQL.
func pgCode(err error) (code, constraint string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code, pgErr.ConstraintName
	}
	return "", ""
}
