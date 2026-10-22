// Package scorestories — этап сюжета ENRICHED → SCORED: go-api (Go #2)
// подбирает компании по признакам и пишет story_company_scores. Ранжирование
// целиком у Go #2 — оркестратор только передаёт ему сюжет и фиксирует переход.
package scorestories

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
const Name = "score"

// Repo — порт хранилища сюжетов.
type Repo interface {
	ClaimStories(ctx context.Context, status domain.PipelineStatus, stage string, limit int) ([]domain.Story, error)
	SetStoryStatus(ctx context.Context, id uuid.UUID, status domain.PipelineStatus) error
}

// Scorer — порт подбора компаний к сюжету (go-api).
type Scorer interface {
	ScoreStory(ctx context.Context, storyID uuid.UUID) (int, error)
}

// Usecase — этап подбора компаний.
type Usecase struct {
	repo     Repo
	scorer   Scorer
	attempts stage.Attempts
	txm      common.TxManager
	events   stage.EventSink
}

// NewUsecase создаёт этап подбора компаний.
func NewUsecase(repo Repo, scorer Scorer, attempts stage.Attempts, txm common.TxManager, events stage.EventSink) *Usecase {
	return &Usecase{repo: repo, scorer: scorer, attempts: attempts, txm: txm, events: events}
}

// Execute отдаёт пачку сюжетов на подбор компаний.
func (uc *Usecase) Execute(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	result := &stage.Result{}
	err := uc.txm.Do(ctx, func(ctx context.Context) error {
		stories, err := uc.repo.ClaimStories(ctx, domain.StatusEnriched, Name, req.Limit)
		if err != nil {
			return err
		}
		for i := range stories {
			if err = uc.scoreOne(ctx, &stories[i], result); err != nil {
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

func (uc *Usecase) scoreOne(ctx context.Context, story *domain.Story, result *stage.Result) error {
	companies, err := uc.scorer.ScoreStory(ctx, story.ID)
	switch {
	case errors.Is(err, domain.ErrDownstreamNotReady):
		result.NotReady = true
		return nil
	case err != nil:
		return stage.RecordFailure(ctx, uc.attempts, uc.events, stage.Failure{
			Kind: stage.KindStory, Stage: Name, ID: story.ID, Cause: err,
			MarkFailed: func(ctx context.Context) error {
				return uc.repo.SetStoryStatus(ctx, story.ID, domain.StatusFailed)
			},
		}, result)
	}

	if err = uc.repo.SetStoryStatus(ctx, story.ID, domain.StatusScored); err != nil {
		return err
	}
	if err = uc.attempts.Clear(ctx, stage.KindStory, story.ID.String(), Name); err != nil {
		return err
	}
	result.Processed++
	stage.Emit(ctx, uc.events, stage.KindStory, story.ID, domain.StatusScored,
		"подобраны компании по признакам", map[string]any{"companies": companies})
	return nil
}
