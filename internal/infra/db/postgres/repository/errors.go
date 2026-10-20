// Package repository содержит pgx-реализации портов usecase.
// Ошибки БД транслируются в domain sentinel здесь, не выше (CLAUDE.md §4).
package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const pgUniqueViolation = "23505"

// isNoRows — запрос не вернул строк.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// isUniqueViolation — нарушение уникального индекса Postgres.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolation
	}
	return false
}
