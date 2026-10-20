package completellm

import (
	"github.com/st1lon/alfa-hack/internal/domain"
)

// Registry — allowlist моделей и маршрутизация задач по моделям.
// Состав задаётся конфигом (configs/llm.yaml), а не кодом: точные id моделей
// уточняются у организаторов, и переезд не должен требовать правки кода.
type Registry struct {
	allowed       map[string]domain.LLMModel
	routing       map[domain.LLMTask]string
	fallbackModel string
}

// NewRegistry собирает реестр из списка разрешённых моделей и карты маршрутизации.
func NewRegistry(models []domain.LLMModel, routing map[domain.LLMTask]string, fallbackModel string) *Registry {
	allowed := make(map[string]domain.LLMModel, len(models))
	for _, model := range models {
		allowed[model.ID] = model
	}
	return &Registry{allowed: allowed, routing: routing, fallbackModel: fallbackModel}
}

// Resolve выбирает модель: явно указанную (если она в allowlist) либо по задаче.
// Модель вне allowlist — это нарушение Ограничения 04, поэтому отказ, а не подмена.
func (r *Registry) Resolve(task domain.LLMTask, explicit string) (domain.LLMModel, error) {
	if explicit != "" {
		model, ok := r.allowed[explicit]
		if !ok {
			return domain.LLMModel{}, domain.ErrModelNotAllowed
		}
		return model, nil
	}

	if routed, ok := r.routing[task]; ok {
		if model, exists := r.allowed[routed]; exists {
			return model, nil
		}
	}

	if model, ok := r.allowed[r.fallbackModel]; ok {
		return model, nil
	}

	return domain.LLMModel{}, domain.ErrUnknownLLMTask
}

// Models возвращает allowlist — для эндпоинта GET /v1/models.
func (r *Registry) Models() []domain.LLMModel {
	models := make([]domain.LLMModel, 0, len(r.allowed))
	for _, model := range r.allowed {
		models = append(models, model)
	}
	return models
}

// Routing возвращает карту «задача → модель».
func (r *Registry) Routing() map[domain.LLMTask]string {
	routing := make(map[domain.LLMTask]string, len(r.routing))
	for task, model := range r.routing {
		routing[task] = model
	}
	return routing
}
