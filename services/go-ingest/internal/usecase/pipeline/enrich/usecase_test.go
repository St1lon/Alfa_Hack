package enrichnews_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	enrichnews "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/enrich"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/stage"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/stage/stagetest"
)

type stubRepo struct {
	news     []domain.News
	statuses map[uuid.UUID]domain.PipelineStatus
	saved    map[uuid.UUID]*domain.Enrichment
}

func newRepo(titles ...string) *stubRepo {
	repo := &stubRepo{statuses: map[uuid.UUID]domain.PipelineStatus{}, saved: map[uuid.UUID]*domain.Enrichment{}}
	for _, title := range titles {
		repo.news = append(repo.news, domain.News{ID: uuid.New(), Title: title, Status: domain.StatusDeduped})
	}
	return repo
}

func (r *stubRepo) ClaimNews(_ context.Context, status domain.PipelineStatus, _ string, limit int) ([]domain.News, error) {
	if status != domain.StatusDeduped {
		return nil, fmt.Errorf("unexpected status %s", status)
	}
	if limit < len(r.news) {
		return r.news[:limit], nil
	}
	return r.news, nil
}

func (r *stubRepo) SetNewsStatus(_ context.Context, id uuid.UUID, status domain.PipelineStatus) error {
	r.statuses[id] = status
	return nil
}

func (r *stubRepo) SaveEnrichment(_ context.Context, e *domain.Enrichment) error {
	r.saved[e.NewsID] = e
	return nil
}

// stubEnricher решает по заголовку: «шум» — шум, «сбой» — ошибка, «нет сервиса» — не готов.
type stubEnricher struct{ calls int }

func (s *stubEnricher) Enrich(_ context.Context, news *domain.News) (*domain.Enrichment, error) {
	s.calls++
	switch news.Title {
	case "сбой":
		return nil, errors.New("py-ml timeout")
	case "нет сервиса":
		return nil, domain.ErrDownstreamNotReady
	case "шум":
		return &domain.Enrichment{NewsID: news.ID, NoiseScore: 0.95, ModelVersion: "test"}, nil
	default:
		return &domain.Enrichment{NewsID: news.ID, NoiseScore: 0.2, ModelVersion: "test"}, nil
	}
}

func newUsecase(repo *stubRepo, enricher *stubEnricher, attempts *stagetest.Attempts, events *stagetest.Events) *enrichnews.Usecase {
	return enrichnews.NewUsecase(repo, enricher, attempts, &stagetest.Tx{}, events,
		enrichnews.Config{NoiseThreshold: 0.8})
}

func TestEnrichSplitsBusinessAndNoise(t *testing.T) {
	t.Parallel()

	repo := newRepo("нефть подорожала", "шум")
	attempts := stagetest.NewAttempts(5)
	events := &stagetest.Events{}
	uc := newUsecase(repo, &stubEnricher{}, attempts, events)

	result, err := uc.Execute(context.Background(), &stage.Request{Limit: 10})

	require.NoError(t, err)
	require.Equal(t, 1, result.Processed)
	require.Equal(t, 1, result.Filtered)
	require.Equal(t, domain.StatusEnriched, repo.statuses[repo.news[0].ID])
	require.Equal(t, domain.StatusFiltered, repo.statuses[repo.news[1].ID], "шум отсекается до любых вызовов моделей")
	require.Len(t, repo.saved, 2, "обогащение сохраняется и у отсеянной новости — для разбора фильтра")
	require.Len(t, attempts.Cleared, 2)
	require.Len(t, events.List, 2)
}

func TestEnrichFailureIsRetriedNotLost(t *testing.T) {
	t.Parallel()

	repo := newRepo("сбой", "нефть подорожала")
	attempts := stagetest.NewAttempts(5)
	uc := newUsecase(repo, &stubEnricher{}, attempts, &stagetest.Events{})

	result, err := uc.Execute(context.Background(), &stage.Request{Limit: 10})

	require.NoError(t, err, "сбой одной новости не валит этап")
	require.Equal(t, 1, result.Failed)
	require.Equal(t, 1, result.Processed)
	require.Equal(t, 1, attempts.Total())
	_, touched := repo.statuses[repo.news[0].ID]
	require.False(t, touched, "новость остаётся в DEDUPED и ждёт ретрая")
}

func TestEnrichMovesToFailedWhenAttemptsRunOut(t *testing.T) {
	t.Parallel()

	repo := newRepo("сбой")
	attempts := stagetest.NewAttempts(1) // первая же неудача — последняя
	events := &stagetest.Events{}
	uc := newUsecase(repo, &stubEnricher{}, attempts, events)

	result, err := uc.Execute(context.Background(), &stage.Request{Limit: 10})

	require.NoError(t, err)
	require.Equal(t, 1, result.Dead)
	require.Equal(t, domain.StatusFailed, repo.statuses[repo.news[0].ID])
	require.Equal(t, domain.StatusFailed, events.List[0].Status)
}

func TestEnrichStopsWhenDownstreamNotReady(t *testing.T) {
	t.Parallel()

	repo := newRepo("нет сервиса", "нефть подорожала", "шум")
	attempts := stagetest.NewAttempts(5)
	enricher := &stubEnricher{}
	uc := newUsecase(repo, enricher, attempts, &stagetest.Events{})

	result, err := uc.Execute(context.Background(), &stage.Request{Limit: 10})

	require.NoError(t, err)
	require.True(t, result.NotReady)
	require.Equal(t, 1, enricher.calls, "дальше по пачке не идём: сервис всё равно лежит")
	require.Zero(t, attempts.Total(), "неготовность соседа не сжигает попытки записей")
	require.Empty(t, repo.statuses)
}

func TestEnrichValidatesLimit(t *testing.T) {
	t.Parallel()

	uc := newUsecase(newRepo(), &stubEnricher{}, stagetest.NewAttempts(5), &stagetest.Events{})
	_, err := uc.Execute(context.Background(), &stage.Request{Limit: 0})
	require.ErrorIs(t, err, domain.ErrValidation)
}
