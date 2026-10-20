package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
)

// LLMRepo — кэш ответов, аудит вызовов и дневной счётчик бюджета.
type LLMRepo struct {
	pool postgres.Querier
}

// NewLLMRepo создаёт репозиторий LLM-Gateway.
func NewLLMRepo(pool postgres.Querier) *LLMRepo {
	return &LLMRepo{pool: pool}
}

// GetCached возвращает кэшированный ответ по хэшу промпта и инкрементит счётчик попаданий.
// Второе значение — был ли кэш-хит.
func (r *LLMRepo) GetCached(ctx context.Context, promptHash string) (json.RawMessage, bool, error) {
	const query = `
		UPDATE llm_cache
		SET hits = hits + 1, last_hit_at = now()
		WHERE prompt_hash = $1
		RETURNING response`

	var response json.RawMessage
	q := postgres.GetQuerier(ctx, r.pool)
	if err := q.QueryRow(ctx, query, promptHash).Scan(&response); err != nil {
		if isNoRows(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("select llm cache: %w", err)
	}
	return response, true, nil
}

// PutCache сохраняет ответ модели под хэшем промпта.
func (r *LLMRepo) PutCache(
	ctx context.Context, promptHash, model string, response json.RawMessage, totalTokens int,
) error {
	const query = `
		INSERT INTO llm_cache (prompt_hash, model, response, total_tokens)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (prompt_hash) DO UPDATE
		SET response = excluded.response, total_tokens = excluded.total_tokens`

	q := postgres.GetQuerier(ctx, r.pool)
	if _, err := q.Exec(ctx, query, promptHash, model, response, totalTokens); err != nil {
		return fmt.Errorf("insert llm cache: %w", err)
	}
	return nil
}

// RecordCall пишет запись аудита вызова модели. Промпты не сохраняем — только хэш
// (Ограничение 06: в журналах нет содержимого и синтетических идентификаторов).
func (r *LLMRepo) RecordCall(ctx context.Context, call *domain.LLMCall) error {
	const query = `
		INSERT INTO llm_calls (
			id, service, actor, task, model, prompt_hash, cache_hit,
			prompt_tokens, completion_tokens, total_tokens, cost_kop,
			latency_ms, status, error, request_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	q := postgres.GetQuerier(ctx, r.pool)
	_, err := q.Exec(ctx, query,
		call.ID, call.Service, nullStr(call.Actor), string(call.Task), call.Model,
		call.PromptHash, call.CacheHit, call.Usage.PromptTokens, call.Usage.CompletionTokens,
		call.Usage.TotalTokens, call.CostKop, call.LatencyMS, string(call.Status),
		nullStr(call.Error), nullStr(call.RequestID),
	)
	if err != nil {
		return fmt.Errorf("insert llm call: %w", err)
	}
	return nil
}

// AddSpend прибавляет расход к дневному счётчику и возвращает состояние дня после записи.
func (r *LLMRepo) AddSpend(ctx context.Context, day time.Time, costKop int64, tokens int) (*domain.LLMBudgetDay, error) {
	const query = `
		INSERT INTO llm_budget_days (day, spent_kop, tokens, calls, updated_at)
		VALUES ($1, $2, $3, 1, now())
		ON CONFLICT (day) DO UPDATE
		SET spent_kop = llm_budget_days.spent_kop + excluded.spent_kop,
		    tokens    = llm_budget_days.tokens + excluded.tokens,
		    calls     = llm_budget_days.calls + 1,
		    updated_at = now()
		RETURNING day, spent_kop, tokens, calls, updated_at`

	var budget domain.LLMBudgetDay
	q := postgres.GetQuerier(ctx, r.pool)
	err := q.QueryRow(ctx, query, day, costKop, tokens).Scan(
		&budget.Day, &budget.SpentKop, &budget.Tokens, &budget.Calls, &budget.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert llm budget: %w", err)
	}
	return &budget, nil
}

// GetBudget возвращает счётчик за день; если записей нет — нулевой счётчик.
func (r *LLMRepo) GetBudget(ctx context.Context, day time.Time) (*domain.LLMBudgetDay, error) {
	const query = `
		SELECT day, spent_kop, tokens, calls, updated_at
		FROM llm_budget_days
		WHERE day = $1`

	var budget domain.LLMBudgetDay
	q := postgres.GetQuerier(ctx, r.pool)
	err := q.QueryRow(ctx, query, day).Scan(
		&budget.Day, &budget.SpentKop, &budget.Tokens, &budget.Calls, &budget.UpdatedAt,
	)
	if err != nil {
		if isNoRows(err) {
			return &domain.LLMBudgetDay{Day: day}, nil
		}
		return nil, fmt.Errorf("select llm budget: %w", err)
	}
	return &budget, nil
}

// SpendByService — разбивка расхода за день по сервисам и людям.
// Нужна, чтобы на третий день не выяснить, что кто-то один выжег дневную квоту.
func (r *LLMRepo) SpendByService(ctx context.Context, day time.Time) (map[string]int64, error) {
	const query = `
		SELECT service, coalesce(sum(cost_kop), 0)
		FROM llm_calls
		WHERE created_at::date = $1
		GROUP BY service`

	q := postgres.GetQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, query, day)
	if err != nil {
		return nil, fmt.Errorf("select spend by service: %w", err)
	}
	defer rows.Close()

	spend := make(map[string]int64)
	for rows.Next() {
		var (
			service string
			kop     int64
		)
		if err = rows.Scan(&service, &kop); err != nil {
			return nil, fmt.Errorf("scan spend row: %w", err)
		}
		spend[service] = kop
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spend rows: %w", err)
	}
	return spend, nil
}

// Ping проверяет живость соединения — используется в /readyz.
func (r *LLMRepo) Ping(ctx context.Context) error {
	const query = `SELECT 1`

	var one int
	q := postgres.GetQuerier(ctx, r.pool)
	if err := q.QueryRow(ctx, query).Scan(&one); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	return nil
}
