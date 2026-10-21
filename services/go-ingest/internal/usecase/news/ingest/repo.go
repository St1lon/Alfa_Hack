package ingestnews

import (
	"context"

	"github.com/st1lon/alfa-hack/internal/domain"
)

//go:generate mockgen -source=repo.go -destination=mocks/repo_mock.go -package=mocks

// NewsRepo — порт хранилища новостей. Только то, что нужно этому usecase.
type NewsRepo interface {
	ExistsByTextHash(ctx context.Context, textHash string) (bool, error)
	Create(ctx context.Context, news *domain.News) error
}

// EventSink — порт журнала событий пайплайна (источник SSE).
type EventSink interface {
	Append(
		ctx context.Context,
		entityKind, entityID string,
		status domain.PipelineStatus,
		service, message string,
		payload any,
	) error
}
