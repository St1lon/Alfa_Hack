// Package postgres содержит подключение к БД, менеджер транзакций и querier-резолвер.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier — общий интерфейс пула и транзакции pgx. Репозитории работают через него.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Pool — Querier, умеющий открывать транзакцию. Реализуется и реальным *pgxpool.Pool,
// и мок-пулом в юнит-тестах репозиториев, поэтому репозитории зависят от интерфейса,
// а не от конкретного типа (Dependency Inversion, CLAUDE.md §1).
type Pool interface {
	Querier
	Begin(ctx context.Context) (pgx.Tx, error)
}

type txCtxKey struct{}

// GetQuerier возвращает активную транзакцию из контекста либо пул по умолчанию.
func GetQuerier(ctx context.Context, pool Querier) Querier {
	if tx, ok := ctx.Value(txCtxKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// TxManager реализует usecase/common.TxManager поверх pgxpool.
type TxManager struct {
	pool Pool
}

// NewTxManager создаёт менеджер транзакций.
func NewTxManager(pool Pool) *TxManager {
	return &TxManager{pool: pool}
}

// Do выполняет fn в транзакции. Вложенный вызов переиспользует активную транзакцию.
func (m *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txCtxKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err = fn(context.WithValue(ctx, txCtxKey{}, tx)); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("%w (rollback: %w)", err, rbErr)
		}
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
