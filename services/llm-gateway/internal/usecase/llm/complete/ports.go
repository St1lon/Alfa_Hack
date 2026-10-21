package completellm

import (
	"context"
	"encoding/json"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
)

//go:generate mockgen -source=ports.go -destination=mocks/ports_mock.go -package=mocks

// ProviderRequest — запрос к провайдеру моделей (OpenAI-совместимый контур).
type ProviderRequest struct {
	Model          string
	Messages       []domain.Message
	Temperature    float64
	MaxTokens      int
	ResponseFormat json.RawMessage
}

// ProviderResponse — ответ провайдера. Этот же тип кладётся в кэш.
type ProviderResponse struct {
	Content      string          `json:"content"`
	FinishReason string          `json:"finish_reason"`
	Usage        domain.LLMUsage `json:"usage"`
}

// Result — что gateway отдаёт вызывающему сервису.
type Result struct {
	Model         string
	Content       string
	FinishReason  string
	Usage         domain.LLMUsage
	CostKop       int64
	CacheHit      bool
	SoftLimitHit  bool
	SpentTodayKop int64
}

// Provider — порт провайдера моделей. Единственная реализация — infra/llmprovider.
type Provider interface {
	Complete(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
}

// Cache — порт кэша ответов по хэшу промпта.
type Cache interface {
	GetCached(ctx context.Context, promptHash string) (json.RawMessage, bool, error)
	PutCache(ctx context.Context, promptHash, model string, response json.RawMessage, totalTokens int) error
}

// Audit — порт журнала вызовов моделей.
type Audit interface {
	RecordCall(ctx context.Context, call *domain.LLMCall) error
}

// Budget — порт дневного счётчика расхода.
type Budget interface {
	GetBudget(ctx context.Context, day time.Time) (*domain.LLMBudgetDay, error)
	AddSpend(ctx context.Context, day time.Time, costKop int64, tokens int) (*domain.LLMBudgetDay, error)
}
