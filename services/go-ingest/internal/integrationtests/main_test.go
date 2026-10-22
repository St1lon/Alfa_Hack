//go:build integration

// Package integrationtests — репозитории go-ingest против настоящего Postgres
// с pgvector и всеми миграциями проекта (testcontainers-go). Запуск:
//
//	go test -tags=integration ./services/go-ingest/internal/integrationtests/...
package integrationtests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/testsupport/pgtest"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/repository"
)

// env — одна база на пакет: контейнер поднимается секунды, а не на каждый тест.
var env *pgtest.Env

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	env, err = pgtest.New(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration env:", err)
		os.Exit(1)
	}

	code := m.Run()
	env.Close(ctx)
	os.Exit(code)
}

// reset очищает таблицы пайплайна, чтобы тесты не видели данных друг друга.
func reset(t *testing.T) {
	t.Helper()
	require.NoError(t, env.Truncate(context.Background(),
		"job_attempts", "dead_letters", "pipeline_events", "story_news",
		"news_enrichment", "news", "stories", "research_chunks", "research_documents"))
}

// insertNews кладёт новость через настоящий репозиторий и переводит в нужный статус.
func insertNews(t *testing.T, title string, status domain.PipelineStatus, publishedAt time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	news := &domain.News{
		ID:          uuid.New(),
		TextHash:    domain.TextHash(title, title+" "+uuid.NewString()),
		Title:       title,
		Body:        "Текст новости для интеграционного теста: " + title,
		Lang:        "ru",
		PublishedAt: &publishedAt,
		FetchedAt:   time.Now().UTC(),
		Status:      domain.StatusRaw,
	}
	require.NoError(t, repository.NewNewsRepo(env.Pool).Create(ctx, news))
	if status != domain.StatusRaw {
		require.NoError(t, repository.NewFlowRepo(env.Pool).SetNewsStatus(ctx, news.ID, status))
	}
	return news.ID
}
