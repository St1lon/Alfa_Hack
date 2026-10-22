package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
)

// ClusterPolicy — параметры склейки новостей в сюжеты в fallback-режиме.
type ClusterPolicy struct {
	// Similarity — минимальное сходство заголовков (pg_trgm, 0..1).
	// Порог строгий: лучше проанализировать перепечатку дважды, чем склеить
	// «ЦБ повысил ставку» с «ЦБ сохранил ставку» в один сюжет.
	Similarity float64
	// Window — насколько старый сюжет ещё может принять новость.
	Window time.Duration
}

// StoryRepo — сюжеты: склейка, захват для этапов, подбор компаний.
type StoryRepo struct {
	pool   postgres.Querier
	policy ClusterPolicy
}

// NewStoryRepo создаёт репозиторий сюжетов.
func NewStoryRepo(pool postgres.Querier, policy ClusterPolicy) *StoryRepo {
	return &StoryRepo{pool: pool, policy: policy}
}

// AssignStory привязывает новость к сюжету: к самому похожему свежему, если
// заголовки достаточно близки, иначе создаёт новый сюжет в статусе ENRICHED.
// Вызывать внутри транзакции. Возвращает id сюжета и признак «создан новый».
//
// SKIP LOCKED в поиске похожего — сознательно: сюжет, который прямо сейчас
// разбирает агент, занят долгой транзакцией этапа analyze. Ждать её нельзя —
// встал бы весь этап склейки, — поэтому такой сюжет пропускаем и заводим новый.
// Цена — изредка лишний разбор перепечатки, а не зависший пайплайн.
func (r *StoryRepo) AssignStory(ctx context.Context, news *domain.News) (uuid.UUID, bool, error) {
	const findSimilar = `
		SELECT id
		FROM stories
		WHERE coalesce(last_published_at, created_at) >= $2::timestamptz - make_interval(secs => $3)
		  AND similarity(title, $1) >= $4
		ORDER BY similarity(title, $1) DESC
		LIMIT 1
		FOR NO KEY UPDATE SKIP LOCKED`

	at := time.Now().UTC()
	if news.PublishedAt != nil {
		at = *news.PublishedAt
	}

	q := postgres.GetQuerier(ctx, r.pool)

	var storyID uuid.UUID
	err := q.QueryRow(ctx, findSimilar, news.Title, at, r.policy.Window.Seconds(), r.policy.Similarity).Scan(&storyID)
	switch {
	case err == nil:
		if err = r.attach(ctx, q, storyID, news.ID, false); err != nil {
			return uuid.Nil, false, err
		}
		const bump = `
			UPDATE stories
			SET sources_count = sources_count + 1,
			    first_published_at = least(first_published_at, $2),
			    last_published_at = greatest(last_published_at, $2),
			    updated_at = now()
			WHERE id = $1`
		if _, err = q.Exec(ctx, bump, storyID, at); err != nil {
			return uuid.Nil, false, fmt.Errorf("update story counters: %w", err)
		}
		return storyID, false, nil
	case !postgres.IsNoRows(err):
		return uuid.Nil, false, fmt.Errorf("find similar story: %w", err)
	}

	statusID, err := domain.StatusEnriched.ID()
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("resolve status: %w", err)
	}

	storyID = uuid.New()
	const create = `
		INSERT INTO stories (id, title, first_published_at, last_published_at, sources_count, status_id)
		VALUES ($1, $2, $3, $3, 1, $4)`
	if _, err = q.Exec(ctx, create, storyID, news.Title, at, statusID); err != nil {
		return uuid.Nil, false, fmt.Errorf("create story: %w", err)
	}
	if err = r.attach(ctx, q, storyID, news.ID, true); err != nil {
		return uuid.Nil, false, err
	}
	return storyID, true, nil
}

// ClaimStories захватывает пачку сюжетов в заданном статусе, у которых не
// отложена следующая попытка этапа. Вызывать внутри транзакции.
func (r *StoryRepo) ClaimStories(
	ctx context.Context, status domain.PipelineStatus, stage string, limit int,
) ([]domain.Story, error) {
	statusID, err := status.ID()
	if err != nil {
		return nil, fmt.Errorf("resolve status %q: %w", status, err)
	}

	const query = `
		SELECT s.id, s.title, s.sources_count, s.first_published_at, s.last_published_at, s.created_at
		FROM stories s
		LEFT JOIN job_attempts j
		       ON j.entity_kind = 'story' AND j.entity_id = s.id::text AND j.stage = $2
		WHERE s.status_id = $1
		  AND (j.next_run_at IS NULL OR j.next_run_at <= now())
		ORDER BY s.created_at
		LIMIT $3
		FOR NO KEY UPDATE OF s SKIP LOCKED`

	q := postgres.GetQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, query, statusID, stage, limit)
	if err != nil {
		return nil, fmt.Errorf("claim stories: %w", err)
	}
	defer rows.Close()

	var stories []domain.Story
	for rows.Next() {
		story := domain.Story{Status: status}
		if err = rows.Scan(&story.ID, &story.Title, &story.SourcesCount,
			&story.FirstPublishedAt, &story.LastPublishedAt, &story.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan claimed story: %w", err)
		}
		stories = append(stories, story)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed stories: %w", err)
	}
	return stories, nil
}

// SetStoryStatus переводит сюжет в новый статус.
func (r *StoryRepo) SetStoryStatus(ctx context.Context, id uuid.UUID, status domain.PipelineStatus) error {
	return setRowStatus(ctx, r.pool, "stories", id, status)
}

// RelevantCompanies — компании, которым сюжет релевантен выше порога,
// самые релевантные первыми, не больше limit. Скоры пишет go-api (Go #2).
func (r *StoryRepo) RelevantCompanies(
	ctx context.Context, storyID uuid.UUID, threshold float64, limit int,
) ([]string, error) {
	const query = `
		SELECT company_id
		FROM story_company_scores
		WHERE story_id = $1 AND relevance_score >= $2
		ORDER BY relevance_score DESC, company_id
		LIMIT $3`

	q := postgres.GetQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, query, storyID, threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("select relevant companies: %w", err)
	}
	defer rows.Close()

	var companies []string
	for rows.Next() {
		var companyID string
		if err = rows.Scan(&companyID); err != nil {
			return nil, fmt.Errorf("scan relevant company: %w", err)
		}
		companies = append(companies, companyID)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relevant companies: %w", err)
	}
	return companies, nil
}

func (r *StoryRepo) attach(ctx context.Context, q postgres.Querier, storyID, newsID uuid.UUID, primary bool) error {
	const link = `INSERT INTO story_news (story_id, news_id, is_primary) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	if _, err := q.Exec(ctx, link, storyID, newsID, primary); err != nil {
		return fmt.Errorf("link news to story: %w", err)
	}
	const setStory = `UPDATE news SET story_id = $2, updated_at = now() WHERE id = $1`
	if _, err := q.Exec(ctx, setStory, newsID, storyID); err != nil {
		return fmt.Errorf("set news story: %w", err)
	}
	return nil
}
