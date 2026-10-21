package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
)

// maxErrorText — сколько символов ошибки храним в job_attempts и dead_letters.
const maxErrorText = 1000

// RetryPolicy — ретраи этапов оркестратора: экспоненциальный бэкофф и потолок попыток.
type RetryPolicy struct {
	MaxAttempts int
	BackoffBase time.Duration
	BackoffMax  time.Duration
}

// AttemptsRepo — учёт неудачных попыток этапов (job_attempts) и dead letters.
type AttemptsRepo struct {
	pool   postgres.Querier
	policy RetryPolicy
}

// NewAttemptsRepo создаёт репозиторий попыток.
func NewAttemptsRepo(pool postgres.Querier, policy RetryPolicy) *AttemptsRepo {
	return &AttemptsRepo{pool: pool, policy: policy}
}

// Fail записывает неудачную попытку этапа и откладывает следующую по
// экспоненте: base, 2·base, 4·base… не больше BackoffMax. Когда попытки
// кончились — запись уходит в dead_letters, и Fail возвращает dead=true:
// вызывающий usecase переводит сущность в FAILED.
func (r *AttemptsRepo) Fail(ctx context.Context, kind, id, stage string, cause error) (bool, error) {
	const upsert = `
		INSERT INTO job_attempts (entity_kind, entity_id, stage, attempts, next_run_at, last_error, updated_at)
		VALUES ($1, $2, $3, 1, now() + make_interval(secs => $4), $5, now())
		ON CONFLICT (entity_kind, entity_id, stage) DO UPDATE
		SET attempts    = job_attempts.attempts + 1,
		    next_run_at = now() + make_interval(secs => least($6, $4 * power(2, job_attempts.attempts))),
		    last_error  = excluded.last_error,
		    updated_at  = now()
		RETURNING attempts`

	errText := truncate(cause.Error())

	var attempts int
	q := postgres.GetQuerier(ctx, r.pool)
	if err := q.QueryRow(ctx, upsert,
		kind, id, stage, r.policy.BackoffBase.Seconds(), errText, r.policy.BackoffMax.Seconds(),
	).Scan(&attempts); err != nil {
		return false, fmt.Errorf("record attempt: %w", err)
	}

	if attempts < r.policy.MaxAttempts {
		return false, nil
	}

	const deadLetter = `
		INSERT INTO dead_letters (entity_kind, entity_id, stage, attempts, last_error)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := q.Exec(ctx, deadLetter, kind, id, stage, attempts, errText); err != nil {
		return false, fmt.Errorf("insert dead letter: %w", err)
	}
	if err := r.Clear(ctx, kind, id, stage); err != nil {
		return false, err
	}
	return true, nil
}

// Clear удаляет счётчик попыток после успешного этапа.
func (r *AttemptsRepo) Clear(ctx context.Context, kind, id, stage string) error {
	const query = `DELETE FROM job_attempts WHERE entity_kind = $1 AND entity_id = $2 AND stage = $3`

	q := postgres.GetQuerier(ctx, r.pool)
	if _, err := q.Exec(ctx, query, kind, id, stage); err != nil {
		return fmt.Errorf("clear attempts: %w", err)
	}
	return nil
}

func truncate(s string) string {
	runes := []rune(s)
	if len(runes) > maxErrorText {
		return string(runes[:maxErrorText])
	}
	return s
}
