// Package newspoller — воркер опроса внешних лент. Driving-адаптер: сам ничего
// не решает про данные, только тянет ленты по расписанию и отдаёт их в usecase.
package newspoller

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/infra/newsfeed"
	ingestnews "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/news/ingest"
)

// Config — параметры воркера.
type Config struct {
	// Tick — как часто воркер просыпается и смотрит, какие ленты пора опросить.
	Tick time.Duration
	// MaxItemsPerPoll — сколько элементов берём из одной ленты за проход.
	MaxItemsPerPoll int
	// HydrateMinBodyLen — если текст из ленты короче, пробуем дотянуть статью
	// со страницы первоисточника: часть лент отдаёт пустой description.
	HydrateMinBodyLen int
	// MaxHydratePerPoll — предел дотягиваний за один проход по одной ленте,
	// чтобы не засыпать сайт запросами.
	MaxHydratePerPoll int
}

// Repo — порт доступа к источникам и к дедупу по URL.
type Repo interface {
	ListSources(ctx context.Context, onlyEnabled bool) ([]domain.NewsSource, error)
	MarkSourcePolled(ctx context.Context, sourceID int16, at time.Time) error
	ExistsByURLHash(ctx context.Context, urlHash string) (bool, error)
}

// FeedFetcher — порт загрузки ленты и страницы новости.
type FeedFetcher interface {
	Fetch(ctx context.Context, feedURL string) ([]newsfeed.Item, error)
	FetchArticle(ctx context.Context, articleURL string) (string, error)
}

// Ingestor — порт приёма новости (usecase ingestnews).
type Ingestor interface {
	Execute(ctx context.Context, req *ingestnews.Request) (*ingestnews.Result, error)
}

// pollStats — что получилось за один проход по ленте.
type pollStats struct {
	fetched    int
	added      int
	duplicates int
	rejected   int
	hydrated   int
}

// Worker опрашивает включённые ленты и складывает новости в БД.
type Worker struct {
	cfg     Config
	repo    Repo
	fetcher FeedFetcher
	ingest  Ingestor
	log     *slog.Logger
}

// New создаёт воркер опроса лент.
func New(cfg Config, repo Repo, fetcher FeedFetcher, ingest Ingestor, log *slog.Logger) *Worker {
	return &Worker{cfg: cfg, repo: repo, fetcher: fetcher, ingest: ingest, log: log}
}

// Run крутит цикл опроса до отмены контекста. Первый проход — сразу на старте,
// чтобы после `docker compose up` новости появились без ожидания тика.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.Tick)
	defer ticker.Stop()

	w.pollDue(ctx)

	for {
		select {
		case <-ctx.Done():
			w.log.Info("news poller stopped")
			return nil
		case <-ticker.C:
			w.pollDue(ctx)
		}
	}
}

// pollDue опрашивает те ленты, у которых истёк их интервал.
func (w *Worker) pollDue(ctx context.Context) {
	sources, err := w.repo.ListSources(ctx, true)
	if err != nil {
		w.log.Error("list news sources failed", slog.String("error", err.Error()))
		return
	}

	now := time.Now().UTC()
	for _, source := range sources {
		if !isDue(source, now) {
			continue
		}
		if err = w.pollSource(ctx, source); err != nil {
			// Ленту могли закрыть, отдать 403 или сломать XML — это норма, идём дальше.
			w.log.Warn("poll source failed",
				slog.String("source", source.Code),
				slog.String("error", err.Error()),
			)
			continue
		}
	}
}

// pollSource тянет одну ленту и отдаёт её элементы в usecase приёма новости.
func (w *Worker) pollSource(ctx context.Context, source domain.NewsSource) error {
	items, err := w.fetcher.Fetch(ctx, source.URL)
	if err != nil {
		return err
	}

	if len(items) > w.cfg.MaxItemsPerPoll {
		items = items[:w.cfg.MaxItemsPerPoll]
	}

	stats := pollStats{fetched: len(items)}
	for i := range items {
		w.handleItem(ctx, source, items[i], &stats)
	}

	if err = w.repo.MarkSourcePolled(ctx, source.ID, time.Now().UTC()); err != nil {
		w.log.Warn("mark source polled failed",
			slog.String("source", source.Code),
			slog.String("error", err.Error()),
		)
	}

	w.log.Info("source polled",
		slog.String("source", source.Code),
		slog.Int("fetched", stats.fetched),
		slog.Int("added", stats.added),
		slog.Int("duplicates", stats.duplicates),
		slog.Int("rejected", stats.rejected),
		slog.Int("hydrated", stats.hydrated),
	)
	return nil
}

// handleItem обрабатывает один элемент ленты: дедуп по URL, дотягивание текста, приём.
func (w *Worker) handleItem(
	ctx context.Context, source domain.NewsSource, item newsfeed.Item, stats *pollStats,
) {
	canonical, err := domain.CanonicalURL(item.Link)
	if err != nil {
		stats.rejected++
		return
	}

	// Дедуп по URL делаем до загрузки страницы: иначе каждый проход по ленте
	// заново качал бы уже известные новости.
	if canonical != "" {
		known, existsErr := w.repo.ExistsByURLHash(ctx, domain.URLHash(canonical))
		if existsErr != nil {
			w.log.Error("check url hash failed",
				slog.String("source", source.Code),
				slog.String("error", existsErr.Error()),
			)
		}
		if known {
			stats.duplicates++
			return
		}
	}

	body := item.Body
	if w.shouldHydrate(body, canonical, stats) {
		article, fetchErr := w.fetcher.FetchArticle(ctx, canonical)
		switch {
		case fetchErr != nil:
			w.log.Debug("fetch article failed",
				slog.String("source", source.Code),
				slog.String("error", fetchErr.Error()),
			)
		case len([]rune(article)) > len([]rune(body)):
			body = article
			stats.hydrated++
		}
	}

	sourceID := source.ID
	result, ingestErr := w.ingest.Execute(ctx, &ingestnews.Request{
		SourceID:    &sourceID,
		URL:         item.Link,
		Title:       item.Title,
		Body:        body,
		Publisher:   fallback(item.Publisher, source.Title),
		PublishedAt: item.PublishedAt,
	})

	switch {
	case ingestErr == nil && result.Duplicate:
		stats.duplicates++
	case ingestErr == nil:
		stats.added++
	case isRejection(ingestErr):
		// Короткий анонс или битая ссылка — отбрасываем молча, это не инцидент.
		stats.rejected++
	default:
		w.log.Error("ingest news failed",
			slog.String("source", source.Code),
			slog.String("error", ingestErr.Error()),
		)
	}
}

// shouldHydrate решает, идти ли за полным текстом на страницу новости.
func (w *Worker) shouldHydrate(body, canonical string, stats *pollStats) bool {
	return canonical != "" &&
		len([]rune(body)) < w.cfg.HydrateMinBodyLen &&
		stats.hydrated < w.cfg.MaxHydratePerPoll
}

func isDue(source domain.NewsSource, now time.Time) bool {
	if source.LastPolledAt == nil {
		return true
	}
	return now.Sub(*source.LastPolledAt) >= source.PollInterval
}

func isRejection(err error) bool {
	return errors.Is(err, domain.ErrInvalidNewsInput) ||
		errors.Is(err, domain.ErrEmptyNewsBody) ||
		errors.Is(err, domain.ErrInvalidNewsURL)
}

func fallback(value, def string) string {
	if value == "" {
		return def
	}
	return value
}
