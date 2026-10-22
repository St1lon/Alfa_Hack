package repository

import (
	"context"
	"fmt"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
)

// StatsRepo — снимок воронки пайплайна для live-панели.
type StatsRepo struct {
	pool postgres.Querier
}

// NewStatsRepo создаёт репозиторий статистики.
func NewStatsRepo(pool postgres.Querier) *StatsRepo {
	return &StatsRepo{pool: pool}
}

// Snapshot собирает счётчики воронки одним походом в базу на группу.
func (r *StatsRepo) Snapshot(ctx context.Context) (*domain.PipelineStats, error) {
	q := postgres.GetQuerier(ctx, r.pool)
	stats := &domain.PipelineStats{}

	var err error
	if stats.NewsByStatus, err = countByStatus(ctx, q, "news"); err != nil {
		return nil, err
	}
	if stats.StoriesByStatus, err = countByStatus(ctx, q, "stories"); err != nil {
		return nil, err
	}

	const totals = `
		SELECT (SELECT count(*) FROM research_documents),
		       (SELECT count(*) FROM research_chunks),
		       (SELECT count(*) FROM news WHERE fetched_at > now() - interval '24 hours'),
		       (SELECT max(fetched_at) FROM news)`
	if err = q.QueryRow(ctx, totals).Scan(
		&stats.ResearchDocs, &stats.ResearchChunks, &stats.NewsLast24h, &stats.LastFetchedAt,
	); err != nil {
		return nil, fmt.Errorf("select pipeline totals: %w", err)
	}
	return stats, nil
}

// countByStatus — сколько строк таблицы в каждом статусе. table — только
// «news» или «stories» из кода этого репозитория, не из ввода.
func countByStatus(ctx context.Context, q postgres.Querier, table string) (map[domain.PipelineStatus]int, error) {
	query := `SELECT s.code, count(*) FROM ` + table + ` t
		JOIN pipeline_statuses s ON s.id = t.status_id GROUP BY s.code`

	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count %s by status: %w", table, err)
	}
	defer rows.Close()

	counts := make(map[domain.PipelineStatus]int)
	for rows.Next() {
		var (
			code  string
			count int
		)
		if err = rows.Scan(&code, &count); err != nil {
			return nil, fmt.Errorf("scan %s status count: %w", table, err)
		}
		counts[domain.PipelineStatus(code)] = count
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s status counts: %w", table, err)
	}
	return counts, nil
}
