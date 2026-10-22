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

// Два воркера одновременно не должны взять одну и ту же новость:
// пока первая транзакция держит свои строки, вторая получает только остальные.
func TestClaimNewsSkipsLockedRows(t *testing.T) {
	reset(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := range 4 {
		insertNews(t, "Новость для захвата "+string(rune('A'+i)), domain.StatusDeduped, now)
	}

	flow := repository.NewFlowRepo(env.Pool)
	txm := postgres.NewTxManager(env.Pool)

	firstIDs := make(chan []uuid.UUID)
	release := make(chan struct{})
	firstDone := make(chan error)

	go func() {
		firstDone <- txm.Do(ctx, func(ctx context.Context) error {
			claimed, err := flow.ClaimNews(ctx, domain.StatusDeduped, "enrich", 2)
			if err != nil {
				return err
			}
			ids := make([]uuid.UUID, 0, len(claimed))
			for _, n := range claimed {
				ids = append(ids, n.ID)
			}
			firstIDs <- ids
			<-release // держим строки захваченными
			return nil
		})
	}()

	held := <-firstIDs
	require.Len(t, held, 2)

	var second []uuid.UUID
	require.NoError(t, txm.Do(ctx, func(ctx context.Context) error {
		claimed, err := flow.ClaimNews(ctx, domain.StatusDeduped, "enrich", 10)
		for _, n := range claimed {
			second = append(second, n.ID)
		}
		return err
	}))

	close(release)
	require.NoError(t, <-firstDone)

	require.Len(t, second, 2, "вторая транзакция видит только незахваченные строки")
	require.NotContains(t, second, held[0])
	require.NotContains(t, second, held[1])
}

// Новость с отложенной попыткой этапа не берётся этим этапом, но видна другим.
func TestClaimNewsRespectsBackoff(t *testing.T) {
	reset(t)
	ctx := context.Background()
	postponed := insertNews(t, "Отложенная новость", domain.StatusDeduped, time.Now().UTC())
	ready := insertNews(t, "Готовая новость", domain.StatusDeduped, time.Now().UTC())

	_, err := env.Pool.Exec(ctx, `
		INSERT INTO job_attempts (entity_kind, entity_id, stage, attempts, next_run_at)
		VALUES ('news', $1, 'enrich', 1, now() + interval '1 hour')`, postponed.String())
	require.NoError(t, err)

	flow := repository.NewFlowRepo(env.Pool)
	claimed, err := flow.ClaimNews(ctx, domain.StatusDeduped, "enrich", 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, ready, claimed[0].ID)

	other, err := flow.ClaimNews(ctx, domain.StatusDeduped, "dedupe", 10)
	require.NoError(t, err)
	require.Len(t, other, 2, "бэкофф одного этапа не мешает другому")
}

// Эмбеддинг пишется в vector(768) текстовым литералом; nil — NULL.
func TestSaveEnrichmentStoresVectorAndJSON(t *testing.T) {
	reset(t)
	ctx := context.Background()
	withVector := insertNews(t, "Новость с эмбеддингом", domain.StatusDeduped, time.Now().UTC())
	withoutVector := insertNews(t, "Новость без эмбеддинга", domain.StatusDeduped, time.Now().UTC())

	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32(i) / 768
	}

	flow := repository.NewFlowRepo(env.Pool)
	require.NoError(t, flow.SaveEnrichment(ctx, &domain.Enrichment{
		NewsID:       withVector,
		Embedding:    embedding,
		Entities:     []domain.Entity{{Type: domain.EntityCountry, Value: "КИТАЙ", Mentions: 2}},
		OKVEDHints:   []domain.OKVEDHint{{Code: "24", Label: "Металлургия", Weight: 1}},
		NoiseScore:   0.1,
		ModelVersion: "pyml-test",
	}))
	require.NoError(t, flow.SaveEnrichment(ctx, &domain.Enrichment{
		NewsID: withoutVector, NoiseScore: 0.5, ModelVersion: "lexical-test",
	}))

	var dims int
	var country string
	require.NoError(t, env.Pool.QueryRow(ctx, `
		SELECT vector_dims(embedding), entities->0->>'value'
		FROM news_enrichment WHERE news_id = $1`, withVector).Scan(&dims, &country))
	require.Equal(t, 768, dims)
	require.Equal(t, "КИТАЙ", country)

	var isNull bool
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT embedding IS NULL FROM news_enrichment WHERE news_id = $1`, withoutVector).Scan(&isNull))
	require.True(t, isNull)
}
