package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const pgUniqueViolation = "23505"

// IsNoRows — запрос не вернул строк.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// IsUniqueViolation — нарушение уникального индекса Postgres.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolation
	}
	return false
}

// NullStr превращает пустую строку в NULL: репозиториям это нужно постоянно.
func NullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
