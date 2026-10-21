package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
)

// ResearchRepo — аналитические материалы банка и их чанки.
type ResearchRepo struct {
	pool postgres.Querier
}

// NewResearchRepo создаёт репозиторий аналитики.
func NewResearchRepo(pool postgres.Querier) *ResearchRepo {
	return &ResearchRepo{pool: pool}
}

// CountChunks — сколько чанков у документа с таким кодом; 0, если документа нет.
func (r *ResearchRepo) CountChunks(ctx context.Context, code string) (int, error) {
	const query = `
		SELECT count(c.id)
		FROM research_documents d
		LEFT JOIN research_chunks c ON c.document_id = d.id
		WHERE d.code = $1`

	var count int
	q := postgres.GetQuerier(ctx, r.pool)
	if err := q.QueryRow(ctx, query, code).Scan(&count); err != nil {
		return 0, fmt.Errorf("count research chunks: %w", err)
	}
	return count, nil
}

// ReplaceDocument сохраняет документ (upsert по коду) и полностью заменяет его чанки.
// Должен вызываться внутри транзакции: иначе между удалением и вставкой
// агент Java может увидеть документ без чанков.
func (r *ResearchRepo) ReplaceDocument(
	ctx context.Context, doc *domain.ResearchDocument, chunks []domain.ResearchChunk,
) error {
	const upsertDoc = `
		INSERT INTO research_documents (code, title, file_name, published_at, pages)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (code) DO UPDATE
		SET title = excluded.title,
		    file_name = excluded.file_name,
		    published_at = excluded.published_at,
		    pages = excluded.pages
		RETURNING id`

	q := postgres.GetQuerier(ctx, r.pool)
	if err := q.QueryRow(ctx, upsertDoc,
		doc.Code, doc.Title, doc.FileName, doc.PublishedAt, doc.Pages,
	).Scan(&doc.ID); err != nil {
		return fmt.Errorf("upsert research document: %w", err)
	}

	const deleteChunks = `DELETE FROM research_chunks WHERE document_id = $1`
	if _, err := q.Exec(ctx, deleteChunks, doc.ID); err != nil {
		return fmt.Errorf("delete research chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil
	}

	// Одна вставка через unnest вместо сотни отдельных INSERT.
	// int32 — под тип integer в Postgres; страницы, индексы и токены далеко от предела.
	const insertChunks = `
		INSERT INTO research_chunks (id, document_id, page, chunk_index, text, tokens, okved_hints, entities)
		SELECT id, $1, page, chunk_index, text, tokens, hints::jsonb, entities::jsonb
		FROM unnest($2::uuid[], $3::int[], $4::int[], $5::text[], $6::int[], $7::text[], $8::text[])
		     AS t(id, page, chunk_index, text, tokens, hints, entities)`

	ids := make([]uuid.UUID, len(chunks))
	pages := make([]int32, len(chunks))
	indexes := make([]int32, len(chunks))
	texts := make([]string, len(chunks))
	tokens := make([]int32, len(chunks))
	hints := make([]string, len(chunks))
	entities := make([]string, len(chunks))
	for i, chunk := range chunks {
		ids[i] = chunk.ID
		pages[i] = int32(chunk.Page)
		indexes[i] = int32(chunk.ChunkIndex)
		texts[i] = chunk.Text
		tokens[i] = int32(chunk.Tokens)
		hints[i], entities[i] = chunkTagsJSON(chunk)
	}

	if _, err := q.Exec(ctx, insertChunks, doc.ID, ids, pages, indexes, texts, tokens, hints, entities); err != nil {
		return fmt.Errorf("insert research chunks: %w", err)
	}
	return nil
}

// chunkTagsJSON — разметка чанка в той же JSON-форме, что news_enrichment.
func chunkTagsJSON(chunk domain.ResearchChunk) (string, string) {
	hints := make([]enrichmentHint, 0, len(chunk.OKVEDHints))
	for _, h := range chunk.OKVEDHints {
		hints = append(hints, enrichmentHint{Code: h.Code, Label: h.Label, Weight: h.Weight})
	}
	entities := make([]enrichmentEntity, 0, len(chunk.Entities))
	for _, e := range chunk.Entities {
		entities = append(entities, enrichmentEntity{Type: string(e.Type), Value: e.Value, Mentions: e.Mentions})
	}
	h, _ := json.Marshal(hints)     //nolint:errchkjson // структуры из строк и чисел маршалятся всегда
	en, _ := json.Marshal(entities) //nolint:errchkjson // структуры из строк и чисел маршалятся всегда
	return string(h), string(en)
}
