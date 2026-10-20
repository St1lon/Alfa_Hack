// Package completellm — единственная точка выхода к языковым моделям.
// Здесь allowlist моделей, маршрутизация по задачам, кэш по хэшу промпта,
// дневной бюджет с hard-стопом и аудит в llm_calls (TEAM_PLAN §5 п.4).
package completellm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// day — длина суток для вычисления текущего бюджетного дня.
const day = 24 * time.Hour

// Usecase — выполнение запроса к модели через gateway.
type Usecase struct {
	registry *Registry
	provider Provider
	cache    Cache
	audit    Audit
	budget   Budget
	limits   Limits
}

// Limits — дневные лимиты расхода на модели.
type Limits struct {
	// SoftLimitKop — порог предупреждения: запросы ещё идут, но в ответе стоит флаг.
	SoftLimitKop int64
	// HardLimitKop — жёсткий стоп: новые вызовы отбиваются ErrBudgetExceeded,
	// кэш при этом продолжает работать (демо не падает).
	HardLimitKop int64
}

// NewUsecase создаёт usecase вызова модели.
func NewUsecase(
	registry *Registry,
	provider Provider,
	cache Cache,
	audit Audit,
	budget Budget,
	limits Limits,
) *Usecase {
	return &Usecase{
		registry: registry,
		provider: provider,
		cache:    cache,
		audit:    audit,
		budget:   budget,
		limits:   limits,
	}
}

// Execute выполняет запрос: кэш → проверка бюджета → провайдер → учёт и аудит.
func (uc *Usecase) Execute(ctx context.Context, req *Request) (*Result, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	model, err := uc.registry.Resolve(req.Task, req.Model)
	if err != nil {
		uc.record(ctx, req, req.Model, "", domain.CallStatusModelDenied, 0, domain.LLMUsage{}, 0, false, err)
		return nil, err
	}

	promptHash := req.promptHash(model.ID)

	if req.UseCache {
		if result, hit, cacheErr := uc.fromCache(ctx, req, model, promptHash); cacheErr == nil && hit {
			return result, nil
		}
	}

	today := time.Now().UTC().Truncate(day)
	spent, err := uc.budget.GetBudget(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("read budget: %w", err)
	}
	if uc.limits.HardLimitKop > 0 && spent.SpentKop >= uc.limits.HardLimitKop {
		uc.record(ctx, req, model.ID, promptHash, domain.CallStatusBudgetExceeded, 0,
			domain.LLMUsage{}, 0, false, domain.ErrBudgetExceeded)
		return nil, domain.ErrBudgetExceeded
	}

	started := time.Now()
	response, err := uc.provider.Complete(ctx, &ProviderRequest{
		Model:          model.ID,
		Messages:       req.Messages,
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
		ResponseFormat: req.ResponseFormat,
	})
	latency := int(time.Since(started).Milliseconds())

	if err != nil {
		uc.record(ctx, req, model.ID, promptHash, domain.CallStatusUpstreamError, latency,
			domain.LLMUsage{}, 0, false, err)
		return nil, fmt.Errorf("%w: %w", domain.ErrUpstreamUnavailable, err)
	}

	costKop := model.CostKop(response.Usage)

	budget, err := uc.budget.AddSpend(ctx, today, costKop, response.Usage.TotalTokens)
	if err != nil {
		return nil, fmt.Errorf("account spend: %w", err)
	}

	if req.UseCache {
		if cached, marshalErr := json.Marshal(response); marshalErr == nil {
			if cacheErr := uc.cache.PutCache(ctx, promptHash, model.ID, cached, response.Usage.TotalTokens); cacheErr != nil {
				return nil, fmt.Errorf("put cache: %w", cacheErr)
			}
		}
	}

	uc.record(ctx, req, model.ID, promptHash, domain.CallStatusOK, latency, response.Usage, costKop, false, nil)

	return &Result{
		Model:         model.ID,
		Content:       response.Content,
		FinishReason:  response.FinishReason,
		Usage:         response.Usage,
		CostKop:       costKop,
		CacheHit:      false,
		SoftLimitHit:  uc.limits.SoftLimitKop > 0 && budget.SpentKop >= uc.limits.SoftLimitKop,
		SpentTodayKop: budget.SpentKop,
	}, nil
}

// fromCache возвращает ответ из кэша, если он там есть. Кэш-хит тоже попадает
// в аудит — иначе непонятно, сколько запросов система реально обслужила.
func (uc *Usecase) fromCache(
	ctx context.Context, req *Request, model domain.LLMModel, promptHash string,
) (*Result, bool, error) {
	raw, hit, err := uc.cache.GetCached(ctx, promptHash)
	if err != nil || !hit {
		return nil, false, err
	}

	var response ProviderResponse
	if err = json.Unmarshal(raw, &response); err != nil {
		return nil, false, fmt.Errorf("unmarshal cached response: %w", err)
	}

	uc.record(ctx, req, model.ID, promptHash, domain.CallStatusOK, 0, response.Usage, 0, true, nil)

	return &Result{
		Model:        model.ID,
		Content:      response.Content,
		FinishReason: response.FinishReason,
		Usage:        response.Usage,
		CostKop:      0,
		CacheHit:     true,
	}, true, nil
}

// record пишет аудит вызова. Сбой аудита не ломает ответ пользователю,
// но логируется как ошибка на уровне транспорта (там есть логгер).
func (uc *Usecase) record(
	ctx context.Context,
	req *Request,
	model, promptHash string,
	status domain.LLMCallStatus,
	latencyMS int,
	usage domain.LLMUsage,
	costKop int64,
	cacheHit bool,
	callErr error,
) {
	errText := ""
	if callErr != nil {
		errText = callErr.Error()
	}

	_ = uc.audit.RecordCall(ctx, &domain.LLMCall{
		ID:         uuid.New(),
		Service:    req.Service,
		Actor:      req.Actor,
		Task:       req.Task,
		Model:      model,
		PromptHash: promptHash,
		CacheHit:   cacheHit,
		Usage:      usage,
		CostKop:    costKop,
		LatencyMS:  latencyMS,
		Status:     status,
		Error:      errText,
		RequestID:  req.RequestID,
	})
}
