package completellm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	completellm "github.com/st1lon/alfa-hack/services/llm-gateway/internal/usecase/llm/complete"
)

func newTestRegistry() *completellm.Registry {
	models := []domain.LLMModel{
		{ID: "gemma-4-26b-a4b", PriceInKopPerM: 500, PriceOutKopPerM: 1000},
		{ID: "glm-5.3-flash", PriceInKopPerM: 1500, PriceOutKopPerM: 3000},
	}
	routing := map[domain.LLMTask]string{
		domain.TaskClassify: "gemma-4-26b-a4b",
		domain.TaskCritic:   "glm-5.3-flash",
		// Намеренно указываем модель вне allowlist: так ловим опечатку в конфиге.
		domain.TaskVision: "qwen3-vl-30b-a3b-instruct",
	}
	return completellm.NewRegistry(models, routing, "gemma-4-26b-a4b")
}

func TestRegistryResolveByTask(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry()

	model, err := registry.Resolve(domain.TaskCritic, "")
	require.NoError(t, err)
	require.Equal(t, "glm-5.3-flash", model.ID)
}

func TestRegistryResolveExplicitModel(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry()

	model, err := registry.Resolve(domain.TaskClassify, "glm-5.3-flash")
	require.NoError(t, err)
	require.Equal(t, "glm-5.3-flash", model.ID)
}

func TestRegistryRejectsModelOutsideAllowlist(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry()

	// Ограничение 04: модель вне списка не вызывается, подмены не делаем.
	_, err := registry.Resolve(domain.TaskAnalyze, "gpt-4o")
	require.ErrorIs(t, err, domain.ErrModelNotAllowed)
}

func TestRegistryFallbackForUnknownTask(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry()

	model, err := registry.Resolve(domain.LLMTask("unknown-task"), "")
	require.NoError(t, err)
	require.Equal(t, "gemma-4-26b-a4b", model.ID)
}

func TestRegistryFallbackWhenRoutedModelIsNotAllowed(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry()

	// Маршрут ведёт на модель, которой нет в allowlist — уходим на fallback,
	// а не на отказ: задача важнее, чем конкретная модель.
	model, err := registry.Resolve(domain.TaskVision, "")
	require.NoError(t, err)
	require.Equal(t, "gemma-4-26b-a4b", model.ID)
}

func TestRegistryModelsAndRouting(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry()

	require.Len(t, registry.Models(), 2)
	require.Equal(t, "glm-5.3-flash", registry.Routing()[domain.TaskCritic])
}
