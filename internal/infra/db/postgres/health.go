package postgres

import (
	"context"
	"fmt"
)

// Health — проверка живости БД для /readyz. Общая для всех сервисов:
// каждому нужен один и тот же SELECT 1, своего репозитория ради этого не заводим.
type Health struct {
	pool Querier
}

// NewHealth создаёт проверку живости поверх пула.
func NewHealth(pool Querier) *Health {
	return &Health{pool: pool}
}

// Ping проверяет, что соединение с базой живо.
func (h *Health) Ping(ctx context.Context) error {
	const query = `SELECT 1`

	var one int
	q := GetQuerier(ctx, h.pool)
	if err := q.QueryRow(ctx, query).Scan(&one); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	return nil
}
