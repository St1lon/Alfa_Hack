//go:build integration

package integrationtests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/services/go-ingest/internal/repository"
)

// Бэкофф растёт экспонентой с потолком, а после последней попытки запись
// уходит в dead_letters и пропадает из job_attempts.
func TestAttemptsBackoffAndDeadLetter(t *testing.T) {
	reset(t)
	ctx := context.Background()

	attempts := repository.NewAttemptsRepo(env.Pool, repository.RetryPolicy{
		MaxAttempts: 4,
		BackoffBase: 10 * time.Second,
		BackoffMax:  25 * time.Second,
	})
	id := uuid.NewString()
	cause := errors.New("py-ml timeout")

	delay := func() float64 {
		var seconds float64
		require.NoError(t, env.Pool.QueryRow(ctx, `
			SELECT extract(epoch FROM next_run_at - now())
			FROM job_attempts WHERE entity_kind = 'news' AND entity_id = $1 AND stage = 'enrich'`,
			id).Scan(&seconds))
		return seconds
	}

	// Ожидания: 10 с, 20 с, потолок 25 с вместо 40 с, четвёртая — смерть.
	for i, want := range []float64{10, 20, 25} {
		dead, err := attempts.Fail(ctx, "news", id, "enrich", cause)
		require.NoError(t, err)
		require.False(t, dead, "попытка %d ещё не последняя", i+1)
		require.InDelta(t, want, delay(), 2, "задержка после попытки %d", i+1)
	}

	dead, err := attempts.Fail(ctx, "news", id, "enrich", cause)
	require.NoError(t, err)
	require.True(t, dead)

	var left, letters int
	var lastError string
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT count(*) FROM job_attempts WHERE entity_id = $1`, id).Scan(&left))
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT count(*), max(last_error) FROM dead_letters WHERE entity_id = $1`, id).Scan(&letters, &lastError))
	require.Zero(t, left, "счётчик попыток убран")
	require.Equal(t, 1, letters)
	require.Equal(t, "py-ml timeout", lastError)
}

// Успех этапа очищает счётчик — следующий сбой снова начнётся с базовой задержки.
func TestAttemptsClear(t *testing.T) {
	reset(t)
	ctx := context.Background()
	attempts := repository.NewAttemptsRepo(env.Pool, repository.RetryPolicy{
		MaxAttempts: 5, BackoffBase: 10 * time.Second, BackoffMax: time.Minute,
	})
	id := uuid.NewString()

	_, err := attempts.Fail(ctx, "story", id, "score", errors.New("boom"))
	require.NoError(t, err)
	require.NoError(t, attempts.Clear(ctx, "story", id, "score"))

	var left int
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT count(*) FROM job_attempts WHERE entity_id = $1`, id).Scan(&left))
	require.Zero(t, left)
}
