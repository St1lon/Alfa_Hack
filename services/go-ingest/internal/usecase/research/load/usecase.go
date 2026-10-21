// Package loadresearch загружает аналитические материалы ЦМИ Альфа-Банка:
// текст по страницам → очистка → чанки с номером страницы → research_chunks.
// Материалы в кейсе — сканы без текстового слоя, текст приходит из OCR.
package loadresearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/usecase/common"
)

const serviceName = "go-ingest"

// Usecase — загрузка аналитики банка.
type Usecase struct {
	repo      ResearchRepo
	extractor PageExtractor
	txm       common.TxManager
	events    EventSink
	chunking  ChunkConfig
}

// NewUsecase создаёт usecase загрузки аналитики.
func NewUsecase(
	repo ResearchRepo,
	extractor PageExtractor,
	txm common.TxManager,
	events EventSink,
	chunking ChunkConfig,
) *Usecase {
	return &Usecase{repo: repo, extractor: extractor, txm: txm, events: events, chunking: chunking}
}

// Execute загружает документы по манифесту. Сбой одного документа не валит
// остальные: он попадает в отчёт с исходом failed, а вызывающий решает,
// считать ли это ошибкой процесса.
func (uc *Usecase) Execute(ctx context.Context, req *Request) (*Result, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	result := &Result{Documents: make([]DocumentReport, 0, len(req.Documents))}
	for _, spec := range req.Documents {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("load research: %w", err)
		}
		result.Documents = append(result.Documents, uc.loadOne(ctx, req, spec))
	}
	return result, nil
}

func (uc *Usecase) loadOne(ctx context.Context, req *Request, spec DocumentSpec) DocumentReport {
	report := DocumentReport{Code: spec.Code}

	if !req.Force {
		existing, err := uc.repo.CountChunks(ctx, spec.Code)
		if err != nil {
			return failed(report, fmt.Errorf("count chunks: %w", err))
		}
		if existing > 0 {
			report.Outcome = OutcomeSkipped
			report.Chunks = existing
			return report
		}
	}

	path := filepath.Join(req.Dir, spec.File)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return failed(report, fmt.Errorf("%w: %s", domain.ErrResearchFileMissing, path))
		}
		return failed(report, fmt.Errorf("stat %s: %w", path, err))
	}

	pages, err := uc.extractor.ExtractPages(ctx, path)
	if err != nil {
		return failed(report, fmt.Errorf("extract pages: %w", err))
	}
	report.Pages = len(pages)

	chunks := uc.buildChunks(pages)
	if len(chunks) == 0 {
		return failed(report, domain.ErrResearchNoText)
	}

	doc := &domain.ResearchDocument{
		Code:        spec.Code,
		Title:       spec.Title,
		FileName:    spec.File,
		PublishedAt: spec.PublishedAt,
		Pages:       len(pages),
	}

	err = uc.txm.Do(ctx, func(ctx context.Context) error {
		return uc.repo.ReplaceDocument(ctx, doc, chunks)
	})
	if err != nil {
		return failed(report, fmt.Errorf("save document: %w", err))
	}

	report.Outcome = OutcomeLoaded
	report.Chunks = len(chunks)
	uc.publishEvent(ctx, doc, len(chunks))
	return report
}

// buildChunks чистит каждую страницу и режет её на чанки со сквозной нумерацией.
func (uc *Usecase) buildChunks(pages []string) []domain.ResearchChunk {
	var chunks []domain.ResearchChunk
	for i, raw := range pages {
		page := i + 1
		for _, draft := range splitChunks(page, cleanPage(raw), uc.chunking) {
			chunks = append(chunks, domain.ResearchChunk{
				ID:         uuid.New(),
				Page:       draft.page,
				ChunkIndex: len(chunks),
				Text:       draft.text,
				Tokens:     estimateTokens(draft.text),
			})
		}
	}
	return chunks
}

// publishEvent пишет событие для live-панели. Сбой журнала не отменяет загрузку.
func (uc *Usecase) publishEvent(ctx context.Context, doc *domain.ResearchDocument, chunks int) {
	if uc.events == nil {
		return
	}
	payload := map[string]any{"title": doc.Title, "pages": doc.Pages, "chunks": chunks}
	_ = uc.events.Append(ctx, "research", doc.Code, "", serviceName, "аналитика банка загружена", payload)
}

func failed(report DocumentReport, err error) DocumentReport {
	report.Outcome = OutcomeFailed
	report.Error = err.Error()
	return report
}
