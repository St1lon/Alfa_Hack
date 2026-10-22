//go:build integration

package integrationtests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/repository"
)

var cluster = repository.ClusterPolicy{Similarity: 0.6, Window: 72 * time.Hour}

func assign(t *testing.T, newsID uuid.UUID, title string, at time.Time) (uuid.UUID, bool) {
	t.Helper()
	ctx := context.Background()
	stories := repository.NewStoryRepo(env.Pool, cluster)

	var (
		storyID uuid.UUID
		created bool
	)
	require.NoError(t, postgres.NewTxManager(env.Pool).Do(ctx, func(ctx context.Context) error {
		var err error
		storyID, created, err = stories.AssignStory(ctx, &domain.News{ID: newsID, Title: title, PublishedAt: &at})
		return err
	}))
	return storyID, created
}

// Перепечатка с тем же заголовком другими словами — тот же сюжет (пара из живого потока Д2).
func TestAssignStoryGluesReprints(t *testing.T) {
	reset(t)
	ctx := context.Background()
	now := time.Now().UTC()

	first := `"Газпром" понизил инвестиции на 2026 год до 1,096 трлн рублей`
	second := `"Газпром" сократил инвестпрограмму на 2026 год до 1,096 трлн рублей`
	firstID := insertNews(t, first, domain.StatusEnriched, now)
	secondID := insertNews(t, second, domain.StatusEnriched, now.Add(time.Hour))

	storyA, createdA := assign(t, firstID, first, now)
	storyB, createdB := assign(t, secondID, second, now.Add(time.Hour))

	require.True(t, createdA)
	require.False(t, createdB, "перепечатка не плодит новый сюжет")
	require.Equal(t, storyA, storyB)

	var sources, links int
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT sources_count FROM stories WHERE id = $1`, storyA).Scan(&sources))
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT count(*) FROM story_news WHERE story_id = $1`, storyA).Scan(&links))
	require.Equal(t, 2, sources)
	require.Equal(t, 2, links)

	var newsStory uuid.UUID
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT story_id FROM news WHERE id = $1`, secondID).Scan(&newsStory))
	require.Equal(t, storyA, newsStory)
}

// Похожие формулировки противоположных событий не склеиваются,
// и старый сюжет за пределами окна новость не принимает.
func TestAssignStoryKeepsDifferentEventsApart(t *testing.T) {
	reset(t)
	now := time.Now().UTC()

	raised := "Банк России повысил ключевую ставку до 15%"
	other := "Минфин разместил ОФЗ на 50 млрд рублей"
	late := "Банк России повысил ключевую ставку до 15%"

	storyA, _ := assign(t, insertNews(t, raised, domain.StatusEnriched, now), raised, now)
	storyB, createdB := assign(t, insertNews(t, other, domain.StatusEnriched, now), other, now)
	storyC, createdC := assign(t, insertNews(t, late, domain.StatusEnriched, now.Add(5*24*time.Hour)), late, now.Add(5*24*time.Hour))

	require.True(t, createdB)
	require.NotEqual(t, storyA, storyB)
	require.True(t, createdC, "за окном 72 ч даже тот же заголовок — новое событие")
	require.NotEqual(t, storyA, storyC)
}
