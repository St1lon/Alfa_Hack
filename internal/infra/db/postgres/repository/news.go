package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
)

// NewsRepo — pgx-реализация портов работы с новостями и источниками.
type NewsRepo struct {
	pool postgres.Querier
}

// NewNewsRepo создаёт репозиторий новостей.
func NewNewsRepo(pool postgres.Querier) *NewsRepo {
	return &NewsRepo{pool: pool}
}

// Create сохраняет новость. Дубликат по url_hash или text_hash транслируется
// в domain.ErrNewsAlreadyExists — это штатный результат дедупа, не ошибка.
func (r *NewsRepo) Create(ctx context.Context, news *domain.News) error {
	statusID, err := news.Status.ID()
	if err != nil {
		return fmt.Errorf("resolve status %q: %w", news.Status, err)
	}

	const query = `
		INSERT INTO news (
			id, source_id, url, canonical_url, url_hash, text_hash,
			title, body, publisher, lang, published_at, fetched_at, status_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	var urlHash *string
	if news.URLHash != "" {
		urlHash = &news.URLHash
	}

	q := postgres.GetQuerier(ctx, r.pool)
	_, err = q.Exec(ctx, query,
		news.ID, news.SourceID, nullStr(news.URL), nullStr(news.CanonicalURL), urlHash, news.TextHash,
		news.Title, news.Body, nullStr(news.Publisher), news.Lang, news.PublishedAt, news.FetchedAt, statusID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrNewsAlreadyExists
		}
		return fmt.Errorf("insert news: %w", err)
	}
	return nil
}

// GetByID возвращает новость по идентификатору.
func (r *NewsRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.News, error) {
	const query = `
		SELECT n.id, n.source_id, coalesce(n.url, ''), coalesce(n.canonical_url, ''),
		       coalesce(n.url_hash, ''), n.text_hash, n.title, n.body,
		       coalesce(n.publisher, ''), n.lang, n.published_at, n.fetched_at,
		       s.code, n.story_id
		FROM news n
		JOIN pipeline_statuses s ON s.id = n.status_id
		WHERE n.id = $1`

	var (
		news   domain.News
		status string
	)
	q := postgres.GetQuerier(ctx, r.pool)
	err := q.QueryRow(ctx, query, id).Scan(
		&news.ID, &news.SourceID, &news.URL, &news.CanonicalURL,
		&news.URLHash, &news.TextHash, &news.Title, &news.Body,
		&news.Publisher, &news.Lang, &news.PublishedAt, &news.FetchedAt,
		&status, &news.StoryID,
	)
	if err != nil {
		if isNoRows(err) {
			return nil, domain.ErrNewsNotFound
		}
		return nil, fmt.Errorf("select news: %w", err)
	}

	news.Status, err = domain.ParsePipelineStatus(status)
	if err != nil {
		return nil, fmt.Errorf("parse status %q: %w", status, err)
	}
	return &news, nil
}

// ExistsByTextHash сообщает, есть ли уже новость с таким хэшем текста.
// Нужен для дешёвой проверки до вставки (чтобы не плодить мусор в логах об ошибках).
func (r *NewsRepo) ExistsByTextHash(ctx context.Context, textHash string) (bool, error) {
	const query = `SELECT exists(SELECT 1 FROM news WHERE text_hash = $1)`

	var exists bool
	q := postgres.GetQuerier(ctx, r.pool)
	if err := q.QueryRow(ctx, query, textHash).Scan(&exists); err != nil {
		return false, fmt.Errorf("check text hash: %w", err)
	}
	return exists, nil
}

// ExistsByURLHash сообщает, есть ли уже новость с таким канонизированным URL.
// Нужен воркеру, чтобы не качать страницу уже известной новости.
func (r *NewsRepo) ExistsByURLHash(ctx context.Context, urlHash string) (bool, error) {
	const query = `SELECT exists(SELECT 1 FROM news WHERE url_hash = $1)`

	if urlHash == "" {
		return false, nil
	}

	var exists bool
	q := postgres.GetQuerier(ctx, r.pool)
	if err := q.QueryRow(ctx, query, urlHash).Scan(&exists); err != nil {
		return false, fmt.Errorf("check url hash: %w", err)
	}
	return exists, nil
}

// UpdateStatus переводит новость в новый статус пайплайна.
func (r *NewsRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.PipelineStatus) error {
	statusID, err := status.ID()
	if err != nil {
		return fmt.Errorf("resolve status %q: %w", status, err)
	}

	const query = `UPDATE news SET status_id = $2, updated_at = now() WHERE id = $1`

	q := postgres.GetQuerier(ctx, r.pool)
	tag, err := q.Exec(ctx, query, id, statusID)
	if err != nil {
		return fmt.Errorf("update news status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNewsNotFound
	}
	return nil
}

// ListSources возвращает источники; onlyEnabled=true — только включённые.
func (r *NewsRepo) ListSources(ctx context.Context, onlyEnabled bool) ([]domain.NewsSource, error) {
	const query = `
		SELECT id, code, title, kind, url, enabled, poll_interval_sec, last_polled_at
		FROM news_sources
		WHERE (NOT $1 OR enabled)
		ORDER BY id`

	q := postgres.GetQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, query, onlyEnabled)
	if err != nil {
		return nil, fmt.Errorf("select news sources: %w", err)
	}
	defer rows.Close()

	var sources []domain.NewsSource
	for rows.Next() {
		var (
			src     domain.NewsSource
			kind    string
			seconds int
		)
		if err = rows.Scan(&src.ID, &src.Code, &src.Title, &kind, &src.URL,
			&src.Enabled, &seconds, &src.LastPolledAt); err != nil {
			return nil, fmt.Errorf("scan news source: %w", err)
		}
		src.Kind = domain.SourceKind(kind)
		src.PollInterval = time.Duration(seconds) * time.Second
		sources = append(sources, src)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate news sources: %w", err)
	}
	return sources, nil
}

// MarkSourcePolled фиксирует время последнего опроса ленты.
func (r *NewsRepo) MarkSourcePolled(ctx context.Context, sourceID int16, at time.Time) error {
	const query = `UPDATE news_sources SET last_polled_at = $2 WHERE id = $1`

	q := postgres.GetQuerier(ctx, r.pool)
	if _, err := q.Exec(ctx, query, sourceID, at); err != nil {
		return fmt.Errorf("mark source polled: %w", err)
	}
	return nil
}

// CountByStatus — сколько новостей в каждом статусе (для /metrics и дашборда команды).
func (r *NewsRepo) CountByStatus(ctx context.Context) (map[string]int, error) {
	const query = `
		SELECT s.code, count(*)
		FROM news n
		JOIN pipeline_statuses s ON s.id = n.status_id
		GROUP BY s.code`

	q := postgres.GetQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count news by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var (
			code  string
			count int
		)
		if err = rows.Scan(&code, &count); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		counts[code] = count
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status counts: %w", err)
	}
	return counts, nil
}

// Ping проверяет живость соединения — используется в /readyz.
func (r *NewsRepo) Ping(ctx context.Context) error {
	const query = `SELECT 1`

	var one int
	q := postgres.GetQuerier(ctx, r.pool)
	if err := q.QueryRow(ctx, query).Scan(&one); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	if one != 1 {
		return errors.New("unexpected ping result")
	}
	return nil
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
