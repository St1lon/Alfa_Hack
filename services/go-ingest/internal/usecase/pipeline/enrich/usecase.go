// Package enrichnews — этап DEDUPED → ENRICHED | FILTERED: эмбеддинг,
// сущности, отрасли и оценка шума. Обогащает py-ml, а пока он не готов —
// словарный fallback внутри Go (USE_ML_FALLBACK=true). Шум отсеивается здесь,
// до любых вызовов моделей: это вторая по силе экономия токенов в системе.
package enrichnews

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/usecase/common"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/stage"
)

// Name — имя этапа в job_attempts и логах.
const Name = "enrich"

// Repo — порт хранилища новостей.
type Repo interface {
	ClaimNews(ctx context.Context, status domain.PipelineStatus, stage string, limit int) ([]domain.News, error)
	SetNewsStatus(ctx context.Context, id uuid.UUID, status domain.PipelineStatus) error
	SaveEnrichment(ctx context.Context, enrichment *domain.Enrichment) error
}

// Enricher — порт обогащения: py-ml или словарный fallback.
type Enricher interface {
	Enrich(ctx context.Context, news *domain.News) (*domain.Enrichment, error)
}

// Config — параметры этапа.
type Config struct {
	// NoiseThreshold — шум не ниже порога → FILTERED.
	NoiseThreshold float64
}

// Usecase — этап обогащения.
type Usecase struct {
	repo     Repo
	enricher Enricher
	attempts stage.Attempts
	txm      common.TxManager
	events   stage.EventSink
	cfg      Config
}

// NewUsecase создаёт этап обогащения.
func NewUsecase(
	repo Repo, enricher Enricher, attempts stage.Attempts,
	txm common.TxManager, events stage.EventSink, cfg Config,
) *Usecase {
	return &Usecase{repo: repo, enricher: enricher, attempts: attempts, txm: txm, events: events, cfg: cfg}
}

// Execute обогащает пачку новостей.
func (uc *Usecase) Execute(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	result := &stage.Result{}
	err := uc.txm.Do(ctx, func(ctx context.Context) error {
		items, err := uc.repo.ClaimNews(ctx, domain.StatusDeduped, Name, req.Limit)
		if err != nil {
			return err
		}
		for i := range items {
			if err = uc.enrichOne(ctx, &items[i], result); err != nil {
				return err
			}
			if result.NotReady {
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("stage %s: %w", Name, err)
	}
	return result, nil
}

// enrichOne обрабатывает одну новость. Возвращает только ошибки базы:
// сбой обогатителя — это ретрай записи, а не провал этапа.
func (uc *Usecase) enrichOne(ctx context.Context, news *domain.News, result *stage.Result) error {
	enrichment, err := uc.enricher.Enrich(ctx, news)
	switch {
	case errors.Is(err, domain.ErrDownstreamNotReady):
		result.NotReady = true
		return nil
	case err != nil:
		return stage.RecordFailure(ctx, uc.attempts, uc.events, stage.Failure{
			Kind: stage.KindNews, Stage: Name, ID: news.ID, Cause: err,
			MarkFailed: func(ctx context.Context) error {
				return uc.repo.SetNewsStatus(ctx, news.ID, domain.StatusFailed)
			},
		}, result)
	}

	if err = uc.repo.SaveEnrichment(ctx, enrichment); err != nil {
		return err
	}

	status, message := domain.StatusEnriched, "обогащено: отрасли и сущности найдены"
	if enrichment.NoiseScore >= uc.cfg.NoiseThreshold {
		status, message = domain.StatusFiltered, "отсеяно как шум для бизнеса"
	}
	if err = uc.repo.SetNewsStatus(ctx, news.ID, status); err != nil {
		return err
	}
	if err = uc.attempts.Clear(ctx, stage.KindNews, news.ID.String(), Name); err != nil {
		return err
	}

	if status == domain.StatusFiltered {
		result.Filtered++
	} else {
		result.Processed++
	}
	stage.Emit(ctx, uc.events, stage.KindNews, news.ID, status, message, eventPayload(enrichment))
	return nil
}

func eventPayload(e *domain.Enrichment) map[string]any {
	hints := make([]string, 0, len(e.OKVEDHints))
	for _, hint := range e.OKVEDHints {
		hints = append(hints, hint.Code+" "+hint.Label)
	}
	return map[string]any{
		"noise_score":   e.NoiseScore,
		"okved_hints":   hints,
		"entities":      len(e.Entities),
		"model_version": e.ModelVersion,
	}
}
