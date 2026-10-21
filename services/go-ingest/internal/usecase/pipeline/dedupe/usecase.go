// Package dedupenews — этап RAW → DEDUPED. Дубликаты по URL и тексту
// отсекаются ещё при приёме новости, поэтому этап только фиксирует переход
// и отдаёт событие в live-панель: «дедуп пройден».
package dedupenews

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/usecase/common"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/stage"
)

// Name — имя этапа в job_attempts и логах.
const Name = "dedupe"

// Repo — порт хранилища новостей.
type Repo interface {
	ClaimNews(ctx context.Context, status domain.PipelineStatus, stage string, limit int) ([]domain.News, error)
	SetNewsStatus(ctx context.Context, id uuid.UUID, status domain.PipelineStatus) error
}

// Usecase — этап дедупа.
type Usecase struct {
	repo   Repo
	txm    common.TxManager
	events stage.EventSink
}

// NewUsecase создаёт этап дедупа.
func NewUsecase(repo Repo, txm common.TxManager, events stage.EventSink) *Usecase {
	return &Usecase{repo: repo, txm: txm, events: events}
}

// Execute переводит пачку новостей из RAW в DEDUPED.
func (uc *Usecase) Execute(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	result := &stage.Result{}
	err := uc.txm.Do(ctx, func(ctx context.Context) error {
		items, err := uc.repo.ClaimNews(ctx, domain.StatusRaw, Name, req.Limit)
		if err != nil {
			return err
		}
		for _, news := range items {
			if err = uc.repo.SetNewsStatus(ctx, news.ID, domain.StatusDeduped); err != nil {
				return err
			}
			result.Processed++
			stage.Emit(ctx, uc.events, stage.KindNews, news.ID, domain.StatusDeduped, "дубликатов нет", nil)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("stage %s: %w", Name, err)
	}
	return result, nil
}
