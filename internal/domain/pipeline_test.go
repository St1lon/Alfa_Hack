package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
)

func TestParsePipelineStatus(t *testing.T) {
	t.Parallel()

	status, err := domain.ParsePipelineStatus("ENRICHED")
	require.NoError(t, err)
	require.Equal(t, domain.StatusEnriched, status)

	_, err = domain.ParsePipelineStatus("SOMETHING_ELSE")
	require.ErrorIs(t, err, domain.ErrUnknownPipelineStatus)
}

func TestPipelineStatusID(t *testing.T) {
	t.Parallel()

	// id должны совпадать со справочником pipeline_statuses из миграции 00001.
	expected := map[domain.PipelineStatus]int16{
		domain.StatusRaw:       1,
		domain.StatusDeduped:   2,
		domain.StatusEnriched:  3,
		domain.StatusScored:    4,
		domain.StatusAnalyzed:  5,
		domain.StatusPublished: 6,
		domain.StatusFailed:    7,
	}

	for status, want := range expected {
		id, err := status.ID()
		require.NoError(t, err)
		require.Equal(t, want, id, status.String())
	}

	_, err := domain.PipelineStatus("NOPE").ID()
	require.ErrorIs(t, err, domain.ErrUnknownPipelineStatus)
}

func TestPipelineTransitions(t *testing.T) {
	t.Parallel()

	// Штатный путь новости.
	require.True(t, domain.StatusRaw.CanTransitionTo(domain.StatusDeduped))
	require.True(t, domain.StatusDeduped.CanTransitionTo(domain.StatusEnriched))
	require.True(t, domain.StatusEnriched.CanTransitionTo(domain.StatusScored))
	require.True(t, domain.StatusScored.CanTransitionTo(domain.StatusAnalyzed))
	require.True(t, domain.StatusAnalyzed.CanTransitionTo(domain.StatusPublished))

	// Перескок через шаг запрещён — иначе дорогая модель получит необогащённые данные.
	require.False(t, domain.StatusRaw.CanTransitionTo(domain.StatusScored))
	require.False(t, domain.StatusEnriched.CanTransitionTo(domain.StatusPublished))

	// Из любого шага можно упасть в FAILED и вернуться на повтор.
	require.True(t, domain.StatusScored.CanTransitionTo(domain.StatusFailed))
	require.True(t, domain.StatusFailed.CanTransitionTo(domain.StatusRaw))

	// PUBLISHED — терминальный статус.
	require.False(t, domain.StatusPublished.CanTransitionTo(domain.StatusAnalyzed))
	require.False(t, domain.StatusPublished.CanTransitionTo(domain.StatusFailed))
}

func TestLLMModelCostKop(t *testing.T) {
	t.Parallel()

	model := domain.LLMModel{ID: "gemma-4-26b-a4b", PriceInKopPerM: 500, PriceOutKopPerM: 1000}

	// 1 млн входных токенов = 500 коп., 1 млн выходных = 1000 коп.
	cost := model.CostKop(domain.LLMUsage{PromptTokens: 1_000_000, CompletionTokens: 1_000_000})
	require.Equal(t, int64(1500), cost)

	// Мелкий вызов округляется вниз до копейки — счётчик не завышает расход.
	require.Equal(t, int64(0), model.CostKop(domain.LLMUsage{PromptTokens: 100, CompletionTokens: 100}))
}
