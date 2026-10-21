package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
)

// FlowRepo — переходы новостей и сюжетов по пайплайну.
//
// Захват строк — FOR NO KEY UPDATE SKIP LOCKED, а не FOR UPDATE: соседние
// сервисы вставляют строки со ссылкой на stories (инсайты Java, скоры Go #2),
// проверка внешнего ключа берёт KEY SHARE, и FOR UPDATE заблокировал бы их
// до конца нашей транзакции, пока мы ждём их же ответа.
type FlowRepo struct {
	pool postgres.Querier
}

// NewFlowRepo создаёт репозиторий переходов.
func NewFlowRepo(pool postgres.Querier) *FlowRepo {
	return &FlowRepo{pool: pool}
}

// ClaimNews захватывает пачку новостей в заданном статусе, у которых не
// отложена следующая попытка этого этапа. Вызывать внутри транзакции.
func (r *FlowRepo) ClaimNews(
	ctx context.Context, status domain.PipelineStatus, stage string, limit int,
) ([]domain.News, error) {
	statusID, err := status.ID()
	if err != nil {
		return nil, fmt.Errorf("resolve status %q: %w", status, err)
	}

	const query = `
		SELECT n.id, n.title, n.body, coalesce(n.publisher, ''), coalesce(n.canonical_url, ''),
		       n.published_at, n.fetched_at, n.story_id
		FROM news n
		LEFT JOIN job_attempts j
		       ON j.entity_kind = 'news' AND j.entity_id = n.id::text AND j.stage = $2
		WHERE n.status_id = $1
		  AND (j.next_run_at IS NULL OR j.next_run_at <= now())
		ORDER BY n.fetched_at
		LIMIT $3
		FOR NO KEY UPDATE OF n SKIP LOCKED`

	q := postgres.GetQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, query, statusID, stage, limit)
	if err != nil {
		return nil, fmt.Errorf("claim news: %w", err)
	}
	defer rows.Close()

	var items []domain.News
	for rows.Next() {
		news := domain.News{Status: status}
		if err = rows.Scan(&news.ID, &news.Title, &news.Body, &news.Publisher, &news.CanonicalURL,
			&news.PublishedAt, &news.FetchedAt, &news.StoryID); err != nil {
			return nil, fmt.Errorf("scan claimed news: %w", err)
		}
		items = append(items, news)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed news: %w", err)
	}
	return items, nil
}

// ClaimUnclusteredNews — обогащённые новости, ещё не привязанные к сюжету.
func (r *FlowRepo) ClaimUnclusteredNews(ctx context.Context, limit int) ([]domain.News, error) {
	statusID, err := domain.StatusEnriched.ID()
	if err != nil {
		return nil, fmt.Errorf("resolve status: %w", err)
	}

	const query = `
		SELECT n.id, n.title, coalesce(n.published_at, n.fetched_at)
		FROM news n
		WHERE n.status_id = $1 AND n.story_id IS NULL
		ORDER BY coalesce(n.published_at, n.fetched_at)
		LIMIT $2
		FOR NO KEY UPDATE OF n SKIP LOCKED`

	q := postgres.GetQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, query, statusID, limit)
	if err != nil {
		return nil, fmt.Errorf("claim unclustered news: %w", err)
	}
	defer rows.Close()

	var items []domain.News
	for rows.Next() {
		var (
			news domain.News
			at   time.Time
		)
		if err = rows.Scan(&news.ID, &news.Title, &at); err != nil {
			return nil, fmt.Errorf("scan unclustered news: %w", err)
		}
		news.PublishedAt = &at
		news.Status = domain.StatusEnriched
		items = append(items, news)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unclustered news: %w", err)
	}
	return items, nil
}

// SetNewsStatus переводит новость в новый статус.
func (r *FlowRepo) SetNewsStatus(ctx context.Context, id uuid.UUID, status domain.PipelineStatus) error {
	return r.setStatus(ctx, "news", id, status)
}

// enrichmentEntity / enrichmentHint — форма JSON в news_enrichment:
// её читает Go #2, поэтому имена полей фиксированы.
type enrichmentEntity struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Mentions int    `json:"mentions"`
}

type enrichmentHint struct {
	Code   string  `json:"code"`
	Label  string  `json:"label"`
	Weight float64 `json:"weight"`
}

// SaveEnrichment сохраняет (или перезаписывает) обогащение новости.
func (r *FlowRepo) SaveEnrichment(ctx context.Context, e *domain.Enrichment) error {
	entities := make([]enrichmentEntity, 0, len(e.Entities))
	for _, entity := range e.Entities {
		entities = append(entities, enrichmentEntity{Type: string(entity.Type), Value: entity.Value, Mentions: entity.Mentions})
	}
	hints := make([]enrichmentHint, 0, len(e.OKVEDHints))
	for _, hint := range e.OKVEDHints {
		hints = append(hints, enrichmentHint{Code: hint.Code, Label: hint.Label, Weight: hint.Weight})
	}

	entitiesJSON, err := json.Marshal(entities)
	if err != nil {
		return fmt.Errorf("marshal entities: %w", err)
	}
	hintsJSON, err := json.Marshal(hints)
	if err != nil {
		return fmt.Errorf("marshal okved hints: %w", err)
	}

	const query = `
		INSERT INTO news_enrichment (news_id, embedding, entities, okved_hints, noise_score, model_version)
		VALUES ($1, $2::vector, $3, $4, $5, $6)
		ON CONFLICT (news_id) DO UPDATE
		SET embedding = excluded.embedding,
		    entities = excluded.entities,
		    okved_hints = excluded.okved_hints,
		    noise_score = excluded.noise_score,
		    model_version = excluded.model_version,
		    created_at = now()`

	q := postgres.GetQuerier(ctx, r.pool)
	if _, err = q.Exec(ctx, query, e.NewsID, vectorLiteral(e.Embedding), entitiesJSON, hintsJSON,
		e.NoiseScore, e.ModelVersion); err != nil {
		return fmt.Errorf("upsert news enrichment: %w", err)
	}
	return nil
}

// vectorLiteral — текстовое представление pgvector: «[0.1,0.2,...]».
// Без отдельной зависимости pgvector-go; nil — NULL (эмбеддинга нет).
func vectorLiteral(v []float32) *string {
	if len(v) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'f', -1, 32))
	}
	b.WriteByte(']')
	s := b.String()
	return &s
}

func (r *FlowRepo) setStatus(ctx context.Context, table string, id uuid.UUID, status domain.PipelineStatus) error {
	return setRowStatus(ctx, r.pool, table, id, status)
}

// setRowStatus — общая смена статуса новости или сюжета.
// table — только «news» или «stories» из кода репозиториев, не из ввода.
func setRowStatus(ctx context.Context, pool postgres.Querier, table string, id uuid.UUID, status domain.PipelineStatus) error {
	statusID, err := status.ID()
	if err != nil {
		return fmt.Errorf("resolve status %q: %w", status, err)
	}

	query := `UPDATE ` + table + ` SET status_id = $2, updated_at = now() WHERE id = $1`

	q := postgres.GetQuerier(ctx, pool)
	tag, err := q.Exec(ctx, query, id, statusID)
	if err != nil {
		return fmt.Errorf("update %s status: %w", table, err)
	}
	if tag.RowsAffected() == 0 {
		if table == "news" {
			return domain.ErrNewsNotFound
		}
		return domain.ErrStoryNotFound
	}
	return nil
}
