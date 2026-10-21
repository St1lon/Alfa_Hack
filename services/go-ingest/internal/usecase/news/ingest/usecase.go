// Package ingestnews принимает новость из ленты или вброса на демо:
// канонизирует, дедуплицирует и сохраняет в статусе RAW.
package ingestnews

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
)

const serviceName = "go-ingest"

// Usecase — приём одной новости.
type Usecase struct {
	repo   NewsRepo
	events EventSink
}

// NewUsecase создаёт usecase приёма новости.
func NewUsecase(repo NewsRepo, events EventSink) *Usecase {
	return &Usecase{repo: repo, events: events}
}

// Execute сохраняет новость. Дубликат — не ошибка: возвращаем Result.Duplicate,
// вызывающий воркер просто идёт дальше по ленте.
func (uc *Usecase) Execute(ctx context.Context, req *Request) (*Result, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	canonical, err := domain.CanonicalURL(req.URL)
	if err != nil {
		return nil, fmt.Errorf("canonicalize url: %w", err)
	}

	textHash := domain.TextHash(req.Title, req.Body)
	exists, err := uc.repo.ExistsByTextHash(ctx, textHash)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if exists {
		return &Result{Duplicate: true}, nil
	}

	news := &domain.News{
		ID:           uuid.New(),
		SourceID:     req.SourceID,
		URL:          req.URL,
		CanonicalURL: canonical,
		URLHash:      domain.URLHash(canonical),
		TextHash:     textHash,
		Title:        req.Title,
		Body:         req.Body,
		Publisher:    req.Publisher,
		Lang:         defaultLang(req.Lang),
		PublishedAt:  req.PublishedAt,
		FetchedAt:    time.Now().UTC(),
		Status:       domain.StatusRaw,
	}

	if err = uc.repo.Create(ctx, news); err != nil {
		// Гонка двух воркеров на одной новости: уникальный индекс — тоже дедуп.
		if errors.Is(err, domain.ErrNewsAlreadyExists) {
			return &Result{Duplicate: true}, nil
		}
		return nil, fmt.Errorf("create news: %w", err)
	}

	uc.publishEvent(ctx, news)

	return &Result{NewsID: news.ID}, nil
}

// publishEvent пишет событие для live-панели. Сбой журнала не отменяет приём новости.
func (uc *Usecase) publishEvent(ctx context.Context, news *domain.News) {
	if uc.events == nil {
		return
	}
	payload := map[string]any{
		"title":     news.Title,
		"publisher": news.Publisher,
		"url":       news.CanonicalURL,
	}
	_ = uc.events.Append(ctx, "news", news.ID.String(), domain.StatusRaw, serviceName, "новость принята", payload)
}

func defaultLang(lang string) string {
	if lang == "" {
		return "ru"
	}
	return lang
}
