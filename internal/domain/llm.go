package domain

import (
	"time"

	"github.com/google/uuid"
)

// LLMTask — тип задачи, по которому gateway выбирает модель (TEAM_PLAN §13).
type LLMTask string

// Задачи маршрутизации. Конкретные модели задаются конфигом, а не кодом.
const (
	TaskClassify  LLMTask = "classify"  // классификация, извлечение, простые преобразования
	TaskExtract   LLMTask = "extract"   //
	TaskAnalyze   LLMTask = "analyze"   // общий анализ и генерация текста
	TaskReason    LLMTask = "reason"    // глубокое рассуждение
	TaskCritic    LLMTask = "critic"    // шаг критика агента
	TaskRelations LLMTask = "relations" // поиск взаимосвязей между событиями
	TaskVision    LLMTask = "vision"    // графики и таблицы из PDF аналитики
)

// LLMCallStatus — исход вызова модели, попадает в аудит llm_calls.
type LLMCallStatus string

// Исходы вызова.
const (
	CallStatusOK             LLMCallStatus = "ok"
	CallStatusUpstreamError  LLMCallStatus = "upstream_error"
	CallStatusBudgetExceeded LLMCallStatus = "budget_exceeded"
	CallStatusModelDenied    LLMCallStatus = "model_denied"
)

// Message — сообщение чата в запросе к модели.
type Message struct {
	Role    string
	Content string
}

// LLMUsage — потребление токенов по данным провайдера.
// json-теги нужны потому, что эта структура уезжает в кэш ответов gateway.
type LLMUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LLMModel — модель из allowlist и её цена для счётчика бюджета.
// Цены в копейках за миллион токенов: деньги целыми, без float (CLAUDE.md §5).
type LLMModel struct {
	ID              string
	PriceInKopPerM  int64
	PriceOutKopPerM int64
}

// CostKop считает стоимость вызова в копейках по потреблению токенов.
func (m LLMModel) CostKop(usage LLMUsage) int64 {
	const perMillion = 1_000_000
	in := int64(usage.PromptTokens) * m.PriceInKopPerM / perMillion
	out := int64(usage.CompletionTokens) * m.PriceOutKopPerM / perMillion
	return in + out
}

// LLMCall — запись аудита одного обращения к модели.
type LLMCall struct {
	ID         uuid.UUID
	Service    string
	Actor      string
	Task       LLMTask
	Model      string
	PromptHash string
	CacheHit   bool
	Usage      LLMUsage
	CostKop    int64
	LatencyMS  int
	Status     LLMCallStatus
	Error      string
	RequestID  string
	CreatedAt  time.Time
}

// LLMBudgetDay — дневной счётчик расхода на модели для soft- и hard-лимита.
type LLMBudgetDay struct {
	Day       time.Time
	SpentKop  int64
	Tokens    int64
	Calls     int
	UpdatedAt time.Time
}
