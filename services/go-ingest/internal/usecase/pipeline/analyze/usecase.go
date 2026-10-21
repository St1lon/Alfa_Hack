// Package analyzestories — этап сюжета SCORED → ANALYZED | FILTERED:
// java-agent строит разбор на каждую релевантную пару «сюжет — компания».
//
// Здесь главный рычаг бюджета: в агента уходят только компании выше порога
// релевантности и не больше MaxCompanies на сюжет. Сюжет, который не
// релевантен никому, отсеивается без единого вызова модели.
package analyzestories

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
const Name = "analyze"

// Repo — порт хранилища сюжетов.
type Repo interface {
	ClaimStories(ctx context.Context, status domain.PipelineStatus, stage string, limit int) ([]domain.Story, error)
	SetStoryStatus(ctx context.Context, id uuid.UUID, status domain.PipelineStatus) error
	RelevantCompanies(ctx context.Context, storyID uuid.UUID, threshold float64, limit int) ([]string, error)
}

// Agent — порт агента (java-agent).
type Agent interface {
	CreateInsight(ctx context.Context, storyID uuid.UUID, companyID string) (string, error)
}

// Config — параметры этапа.
type Config struct {
	// RelevanceThreshold — ниже этого relevance_score компания в агента не уходит.
	RelevanceThreshold float64
	// MaxCompanies — потолок разборов на один сюжет (бюджет моделей).
	MaxCompanies int
}

// Usecase — этап разбора.
type Usecase struct {
	repo     Repo
	agent    Agent
	attempts stage.Attempts
	txm      common.TxManager
	events   stage.EventSink
	cfg      Config
}

// NewUsecase создаёт этап разбора.
func NewUsecase(
	repo Repo, agent Agent, attempts stage.Attempts,
	txm common.TxManager, events stage.EventSink, cfg Config,
) *Usecase {
	return &Usecase{repo: repo, agent: agent, attempts: attempts, txm: txm, events: events, cfg: cfg}
}

// Execute отдаёт пачку сюжетов агенту.
func (uc *Usecase) Execute(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	result := &stage.Result{}
	err := uc.txm.Do(ctx, func(ctx context.Context) error {
		stories, err := uc.repo.ClaimStories(ctx, domain.StatusScored, Name, req.Limit)
		if err != nil {
			return err
		}
		for i := range stories {
			if err = uc.analyzeOne(ctx, &stories[i], result); err != nil {
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

func (uc *Usecase) analyzeOne(ctx context.Context, story *domain.Story, result *stage.Result) error {
	companies, err := uc.repo.RelevantCompanies(ctx, story.ID, uc.cfg.RelevanceThreshold, uc.cfg.MaxCompanies)
	if err != nil {
		return err
	}

	if len(companies) == 0 {
		if err = uc.repo.SetStoryStatus(ctx, story.ID, domain.StatusFiltered); err != nil {
			return err
		}
		result.Filtered++
		stage.Emit(ctx, uc.events, stage.KindStory, story.ID, domain.StatusFiltered,
			"не релевантен ни одной компании выше порога", nil)
		return nil
	}

	insights := make([]string, 0, len(companies))
	for _, companyID := range companies {
		// Агент идемпотентен по паре «сюжет — компания», поэтому после сбоя
		// повторяем все пары сюжета, а не помним, какие уже прошли.
		insightID, callErr := uc.agent.CreateInsight(ctx, story.ID, companyID)
		switch {
		case errors.Is(callErr, domain.ErrDownstreamNotReady):
			result.NotReady = true
			return nil
		case callErr != nil:
			return stage.RecordFailure(ctx, uc.attempts, uc.events, stage.Failure{
				Kind: stage.KindStory, Stage: Name, ID: story.ID,
				Cause: fmt.Errorf("company %s: %w", companyID, callErr),
				MarkFailed: func(ctx context.Context) error {
					return uc.repo.SetStoryStatus(ctx, story.ID, domain.StatusFailed)
				},
			}, result)
		}
		insights = append(insights, insightID)
	}

	if err = uc.repo.SetStoryStatus(ctx, story.ID, domain.StatusAnalyzed); err != nil {
		return err
	}
	if err = uc.attempts.Clear(ctx, stage.KindStory, story.ID.String(), Name); err != nil {
		return err
	}
	result.Processed++
	stage.Emit(ctx, uc.events, stage.KindStory, story.ID, domain.StatusAnalyzed,
		"агент построил разборы", map[string]any{"companies": companies, "insights": insights})
	return nil
}
