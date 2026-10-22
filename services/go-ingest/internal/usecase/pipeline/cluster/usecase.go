// Package clusternews — склейка обогащённых новостей в сюжеты. Перепечатки
// одной новости становятся одним сюжетом: дорогая модель вызывается на сюжет,
// а не на каждую из шести копий. Fallback-склейка — по сходству заголовков.
package clusternews

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/usecase/common"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/stage"
)

// Name — имя этапа в логах.
const Name = "cluster"

// Repo — порт хранилища новостей.
type Repo interface {
	ClaimUnclusteredNews(ctx context.Context, limit int) ([]domain.News, error)
}

// StoryAssigner — порт привязки новости к сюжету.
type StoryAssigner interface {
	AssignStory(ctx context.Context, news *domain.News) (uuid.UUID, bool, error)
}

// Usecase — этап склейки.
type Usecase struct {
	repo     Repo
	assigner StoryAssigner
	txm      common.TxManager
	events   stage.EventSink
}

// NewUsecase создаёт этап склейки.
func NewUsecase(repo Repo, assigner StoryAssigner, txm common.TxManager, events stage.EventSink) *Usecase {
	return &Usecase{repo: repo, assigner: assigner, txm: txm, events: events}
}

// Execute привязывает пачку новостей к сюжетам.
func (uc *Usecase) Execute(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	result := &stage.Result{}
	err := uc.txm.Do(ctx, func(ctx context.Context) error {
		items, err := uc.repo.ClaimUnclusteredNews(ctx, req.Limit)
		if err != nil {
			return err
		}
		for i := range items {
			news := &items[i]
			storyID, created, assignErr := uc.assigner.AssignStory(ctx, news)
			if assignErr != nil {
				return assignErr
			}
			result.Processed++

			payload := map[string]any{"news_id": news.ID.String(), "title": news.Title}
			if created {
				stage.Emit(ctx, uc.events, stage.KindStory, storyID, domain.StatusEnriched, "новый сюжет", payload)
				continue
			}
			stage.Emit(ctx, uc.events, stage.KindStory, storyID, "", "в сюжет добавлен ещё один источник", payload)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("stage %s: %w", Name, err)
	}
	return result, nil
}
