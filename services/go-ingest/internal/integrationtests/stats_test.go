//go:build integration

package integrationtests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/repository"
)

// Снимок воронки — то, что видит live-панель: счётчики по статусам и объём базы знаний.
func TestStatsSnapshotCountsFunnel(t *testing.T) {
	reset(t)
	ctx := context.Background()
	now := time.Now().UTC()

	empty, err := repository.NewStatsRepo(env.Pool).Snapshot(ctx)
	require.NoError(t, err)
	require.Empty(t, empty.NewsByStatus)
	require.Nil(t, empty.LastFetchedAt, "пустая база — времени последней новости нет")

	insertNews(t, "Новость, которая ждёт дедупа", domain.StatusRaw, now)
	insertNews(t, "Шумная новость про футбол", domain.StatusFiltered, now)
	insertNews(t, "Ещё одна шумная новость", domain.StatusFiltered, now)
	insertNews(t, "ЦБ сохранил ключевую ставку", domain.StatusEnriched, now)

	stats, err := repository.NewStatsRepo(env.Pool).Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, map[domain.PipelineStatus]int{
		domain.StatusRaw: 1, domain.StatusFiltered: 2, domain.StatusEnriched: 1,
	}, stats.NewsByStatus)
	require.Equal(t, 4, stats.NewsLast24h)
	require.Zero(t, stats.ResearchChunks)
	require.NotNil(t, stats.LastFetchedAt)
}

// Без явной позиции SSE начинает с хвоста журнала — ему нужен последний id.
func TestLatestEventIDFollowsJournal(t *testing.T) {
	reset(t)
	ctx := context.Background()
	events := repository.NewPipelineRepo(env.Pool)

	latest, err := events.LatestEventID(ctx)
	require.NoError(t, err)
	require.Zero(t, latest, "пустой журнал — ноль, а не ошибка")

	for range 3 {
		require.NoError(t, events.Append(ctx, "news", "id", domain.StatusRaw, "go-ingest", "новость принята", nil))
	}
	latest, err = events.LatestEventID(ctx)
	require.NoError(t, err)

	tail, err := events.ListAfter(ctx, latest-1, 10)
	require.NoError(t, err)
	require.Len(t, tail, 1)
	require.Equal(t, latest, tail[0].ID)
}
