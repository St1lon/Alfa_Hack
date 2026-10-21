// Package gatewayhttp — HTTP-контур llm-gateway: вызов модели, allowlist,
// состояние дневного бюджета. Ключ провайдера наружу не отдаётся никогда.
package gatewayhttp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
	httpx "github.com/st1lon/alfa-hack/internal/transport/http"
	"github.com/st1lon/alfa-hack/internal/transport/http/apierrors"
	completellm "github.com/st1lon/alfa-hack/services/llm-gateway/internal/usecase/llm/complete"
)

// day — длина суток для вычисления текущего бюджетного дня.
const day = 24 * time.Hour

// Completer — порт вызова модели (usecase completellm).
type Completer interface {
	Execute(ctx context.Context, req *completellm.Request) (*completellm.Result, error)
}

// BudgetReader — порт чтения дневного расхода.
type BudgetReader interface {
	GetBudget(ctx context.Context, day time.Time) (*domain.LLMBudgetDay, error)
	SpendByService(ctx context.Context, day time.Time) (map[string]int64, error)
}

// Pinger — порт проверки живости БД.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps — зависимости HTTP-контура gateway.
type Deps struct {
	Complete Completer
	Registry *completellm.Registry
	Budget   BudgetReader
	DB       Pinger
	Limits   completellm.Limits
	Log      *slog.Logger
}

// Handlers реализует эндпоинты llm-gateway.
type Handlers struct {
	deps Deps
}

// completeRequest — тело POST /v1/complete.
type completeRequest struct {
	Service        string          `json:"service"`
	Actor          string          `json:"actor"`
	Task           string          `json:"task"`
	Model          string          `json:"model"`
	Messages       []messageDTO    `json:"messages"`
	Temperature    float64         `json:"temperature"`
	MaxTokens      int             `json:"max_tokens"`
	ResponseFormat json.RawMessage `json:"response_format"`
	// Cache: по умолчанию кэш включён — его отключают только осознанно.
	Cache *bool `json:"cache"`
}

type messageDTO struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// completeResponse — ответ gateway вызывающему сервису.
type completeResponse struct {
	Model         string   `json:"model"`
	Content       string   `json:"content"`
	FinishReason  string   `json:"finish_reason,omitempty"`
	Usage         usageDTO `json:"usage"`
	CostKop       int64    `json:"cost_kop"`
	CacheHit      bool     `json:"cache_hit"`
	SoftLimitHit  bool     `json:"soft_limit_hit"`
	SpentTodayKop int64    `json:"spent_today_kop"`
}

type usageDTO struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// modelsResponse — allowlist и маршрутизация задач.
type modelsResponse struct {
	Models  []modelDTO        `json:"models"`
	Routing map[string]string `json:"routing"`
}

type modelDTO struct {
	ID              string `json:"id"`
	PriceInKopPerM  int64  `json:"price_in_kop_per_mtok"`
	PriceOutKopPerM int64  `json:"price_out_kop_per_mtok"`
}

// budgetResponse — состояние дневного бюджета для дашборда команды.
type budgetResponse struct {
	Day            string           `json:"day"`
	SpentKop       int64            `json:"spent_kop"`
	Tokens         int64            `json:"tokens"`
	Calls          int              `json:"calls"`
	SoftLimitKop   int64            `json:"soft_limit_kop"`
	HardLimitKop   int64            `json:"hard_limit_kop"`
	RemainingKop   int64            `json:"remaining_kop"`
	SpendByService map[string]int64 `json:"spend_by_service"`
}

// healthResponse — ответ health- и readiness-проб.
type healthResponse struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// NewHandlers создаёт обработчики llm-gateway.
func NewHandlers(deps Deps) *Handlers {
	return &Handlers{deps: deps}
}

// Complete выполняет запрос к модели: POST /v1/complete.
func (h *Handlers) Complete(w http.ResponseWriter, r *http.Request) {
	var body completeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteJSON(w, h.deps.Log, http.StatusBadRequest, apierrors.BadRequest("malformed json body"))
		return
	}

	messages := make([]domain.Message, 0, len(body.Messages))
	for _, message := range body.Messages {
		messages = append(messages, domain.Message{Role: message.Role, Content: message.Content})
	}

	useCache := true
	if body.Cache != nil {
		useCache = *body.Cache
	}

	result, err := h.deps.Complete.Execute(r.Context(), &completellm.Request{
		Service:        body.Service,
		Actor:          body.Actor,
		Task:           domain.LLMTask(body.Task),
		Model:          body.Model,
		Messages:       messages,
		Temperature:    body.Temperature,
		MaxTokens:      body.MaxTokens,
		ResponseFormat: body.ResponseFormat,
		UseCache:       useCache,
		RequestID:      r.Header.Get("X-Request-Id"),
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}

	httpx.WriteJSON(w, h.deps.Log, http.StatusOK, completeResponse{
		Model:        result.Model,
		Content:      result.Content,
		FinishReason: result.FinishReason,
		Usage: usageDTO{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
		CostKop:       result.CostKop,
		CacheHit:      result.CacheHit,
		SoftLimitHit:  result.SoftLimitHit,
		SpentTodayKop: result.SpentTodayKop,
	})
}

// Models отдаёт allowlist и маршрутизацию: GET /v1/models.
func (h *Handlers) Models(w http.ResponseWriter, _ *http.Request) {
	models := h.deps.Registry.Models()
	dto := make([]modelDTO, 0, len(models))
	for _, model := range models {
		dto = append(dto, modelDTO{
			ID:              model.ID,
			PriceInKopPerM:  model.PriceInKopPerM,
			PriceOutKopPerM: model.PriceOutKopPerM,
		})
	}

	routing := make(map[string]string)
	for task, model := range h.deps.Registry.Routing() {
		routing[string(task)] = model
	}

	httpx.WriteJSON(w, h.deps.Log, http.StatusOK, modelsResponse{Models: dto, Routing: routing})
}

// Budget отдаёт расход за сегодня: GET /v1/budget.
func (h *Handlers) Budget(w http.ResponseWriter, r *http.Request) {
	today := time.Now().UTC().Truncate(day)

	budget, err := h.deps.Budget.GetBudget(r.Context(), today)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}

	byService, err := h.deps.Budget.SpendByService(r.Context(), today)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}

	remaining := h.deps.Limits.HardLimitKop - budget.SpentKop
	if remaining < 0 {
		remaining = 0
	}

	httpx.WriteJSON(w, h.deps.Log, http.StatusOK, budgetResponse{
		Day:            today.Format(time.DateOnly),
		SpentKop:       budget.SpentKop,
		Tokens:         budget.Tokens,
		Calls:          budget.Calls,
		SoftLimitKop:   h.deps.Limits.SoftLimitKop,
		HardLimitKop:   h.deps.Limits.HardLimitKop,
		RemainingKop:   remaining,
		SpendByService: byService,
	})
}

// Healthz — liveness: процесс жив.
func (h *Handlers) Healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, h.deps.Log, http.StatusOK, healthResponse{Status: "ok"})
}

// Readyz — readiness: есть коннект к БД.
func (h *Handlers) Readyz(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.DB.Ping(r.Context()); err != nil {
		httpx.WriteJSON(w, h.deps.Log, http.StatusServiceUnavailable, healthResponse{
			Status: "unavailable",
			Detail: "database is not reachable",
		})
		return
	}
	httpx.WriteJSON(w, h.deps.Log, http.StatusOK, healthResponse{Status: "ok"})
}

func (h *Handlers) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	status, body := apierrors.Map(err)
	if status >= http.StatusInternalServerError {
		h.deps.Log.ErrorContext(r.Context(), "request failed",
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
	}
	httpx.WriteJSON(w, h.deps.Log, status, body)
}
