// Package common держит код, переиспользуемый несколькими usecase (CLAUDE.md §3).
package common

import "context"

// TxManager — порт работы с транзакциями. Реализация — postgres.TxManager.
type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
