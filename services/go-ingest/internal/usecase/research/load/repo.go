package loadresearch

import (
	"context"

	"github.com/st1lon/alfa-hack/internal/domain"
)

//go:generate mockgen -source=repo.go -destination=mocks/repo_mock.go -package=mocks

// ResearchRepo — порт хранилища аналитики.
type ResearchRepo interface {
	// CountChunks — сколько чанков уже лежит у документа с таким кодом (0, если документа нет).
	CountChunks(ctx context.Context, code string) (int, error)
	// ReplaceDocument сохраняет документ и целиком заменяет его чанки.
	ReplaceDocument(ctx context.Context, doc *domain.ResearchDocument, chunks []domain.ResearchChunk) error
}

// PageExtractor — порт извлечения текста по страницам. Элемент i — страница i+1.
type PageExtractor interface {
	ExtractPages(ctx context.Context, path string) ([]string, error)
}

// Classifier — порт классификации текста чанка (отрасли, страны, сырьё,
// риск-теги). Необязательный: без него чанки сохраняются без разметки.
type Classifier interface {
	Classify(text string) ([]domain.OKVEDHint, []domain.Entity)
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
