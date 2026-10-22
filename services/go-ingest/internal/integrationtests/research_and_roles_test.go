//go:build integration

package integrationtests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/repository"
)

func chunk(page, index int, text string) domain.ResearchChunk {
	return domain.ResearchChunk{
		ID: uuid.New(), Page: page, ChunkIndex: index, Text: text, Tokens: len(text) / 3,
		OKVEDHints: []domain.OKVEDHint{{Code: "64", Label: "Финансы и банки", Weight: 1}},
		Entities:   []domain.Entity{{Type: domain.EntityRiskTag, Value: "rate_risk", Mentions: 1}},
	}
}

// Повторная загрузка документа заменяет чанки целиком, а разметка
// находится тем же запросом, которым будет искать агент.
func TestReplaceDocumentReplacesChunksAndKeepsTags(t *testing.T) {
	reset(t)
	ctx := context.Background()
	research := repository.NewResearchRepo(env.Pool)
	txm := postgres.NewTxManager(env.Pool)

	save := func(title string, chunks []domain.ResearchChunk) {
		doc := &domain.ResearchDocument{Code: "policy_rate", Title: title, FileName: "rate.pdf", Pages: 4}
		require.NoError(t, txm.Do(ctx, func(ctx context.Context) error {
			return research.ReplaceDocument(ctx, doc, chunks)
		}))
	}

	save("Ставка (черновик)", []domain.ResearchChunk{chunk(1, 0, "а"), chunk(2, 1, "б"), chunk(3, 2, "в")})
	save("Ключевая ставка ЦБ РФ", []domain.ResearchChunk{chunk(1, 0, "новый текст"), chunk(4, 1, "ещё")})

	count, err := research.CountChunks(ctx, "policy_rate")
	require.NoError(t, err)
	require.Equal(t, 2, count, "старые чанки удалены, а не дописаны")

	var title string
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT title FROM research_documents WHERE code = 'policy_rate'`).Scan(&title))
	require.Equal(t, "Ключевая ставка ЦБ РФ", title)

	var tagged int
	require.NoError(t, env.Pool.QueryRow(ctx, `
		SELECT count(*) FROM research_chunks
		WHERE entities @> '[{"type": "RISK_TAG", "value": "rate_risk"}]'`).Scan(&tagged))
	require.Equal(t, 2, tagged)

	missing, err := research.CountChunks(ctx, "no_such_doc")
	require.NoError(t, err)
	require.Zero(t, missing)
}

func requireDenied(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr, "ждали ошибку Postgres")
	require.Equal(t, "42501", pgErr.Code, "ждали insufficient_privilege")
}

// Роли из миграции 00004 — то, на чём держится раздел про права в SECURITY.md.
// Если кто-то случайно выдаст лишнее, этот тест упадёт.
func TestServiceRolesIsolation(t *testing.T) {
	reset(t)
	ctx := context.Background()
	insertNews(t, "Новость для проверки ролей", domain.StatusDeduped, time.Now().UTC())

	gateway, err := env.ConnectAs(ctx, "svc_gateway")
	require.NoError(t, err)
	defer gateway.Close()

	var n int
	require.NoError(t, gateway.QueryRow(ctx, `SELECT count(*) FROM llm_calls`).Scan(&n), "своя таблица доступна")
	requireDenied(t, gateway.QueryRow(ctx, `SELECT count(*) FROM companies`).Scan(&n))
	requireDenied(t, gateway.QueryRow(ctx, `SELECT count(*) FROM news`).Scan(&n))

	ml, err := env.ConnectAs(ctx, "svc_ml")
	require.NoError(t, err)
	defer ml.Close()
	require.NoError(t, ml.QueryRow(ctx, `SELECT count(*) FROM news`).Scan(&n))
	_, err = ml.Exec(ctx, `DELETE FROM news_enrichment WHERE false`)
	requireDenied(t, err)

	api, err := env.ConnectAs(ctx, "svc_api")
	require.NoError(t, err)
	defer api.Close()
	_, err = api.Exec(ctx, `UPDATE news SET status_id = status_id WHERE false`)
	requireDenied(t, err)

	// go-ingest под своей ролью выполняет реальный захват пачки, как оркестратор.
	ingest, err := env.ConnectAs(ctx, "svc_ingest")
	require.NoError(t, err)
	defer ingest.Close()
	flow := repository.NewFlowRepo(ingest)
	require.NoError(t, postgres.NewTxManager(ingest).Do(ctx, func(ctx context.Context) error {
		claimed, claimErr := flow.ClaimNews(ctx, domain.StatusDeduped, "enrich", 10)
		if claimErr != nil {
			return claimErr
		}
		require.Len(t, claimed, 1)
		return flow.SetNewsStatus(ctx, claimed[0].ID, domain.StatusEnriched)
	}))
}
