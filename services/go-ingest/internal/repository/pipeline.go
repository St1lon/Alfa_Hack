package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
)

// PipelineRepo — журнал событий пайплайна (источник SSE-стрима).
type PipelineRepo struct {
	pool postgres.Querier
}

// NewPipelineRepo создаёт репозиторий событий пайплайна.
func NewPipelineRepo(pool postgres.Querier) *PipelineRepo {
	return &PipelineRepo{pool: pool}
}

// Append добавляет событие. Статус может быть пустым (технические события).
func (r *PipelineRepo) Append(
	ctx context.Context,
	entityKind, entityID string,
	status domain.PipelineStatus,
	service, message string,
	payload any,
) error {
	var statusID *int16
	if status != "" {
		id, err := status.ID()
		if err != nil {
			return fmt.Errorf("resolve status %q: %w", status, err)
		}
		statusID = &id
	}

	raw := []byte("{}")
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal event payload: %w", err)
		}
		raw = encoded
	}

	const query = `
		INSERT INTO pipeline_events (entity_kind, entity_id, status_id, service, message, payload)
		VALUES ($1, $2, $3, $4, $5, $6)`

	q := postgres.GetQuerier(ctx, r.pool)
	if _, err := q.Exec(ctx, query, entityKind, entityID, statusID, service, postgres.NullStr(message), raw); err != nil {
		return fmt.Errorf("insert pipeline event: %w", err)
	}
	return nil
}

// ListAfter возвращает события с id больше afterID — основа SSE-стрима.
func (r *PipelineRepo) ListAfter(ctx context.Context, afterID int64, limit int) ([]domain.PipelineEvent, error) {
	const query = `
		SELECT e.id, e.entity_kind, e.entity_id, coalesce(s.code, ''), e.service,
		       coalesce(e.message, ''), e.payload, e.created_at
		FROM pipeline_events e
		LEFT JOIN pipeline_statuses s ON s.id = e.status_id
		WHERE e.id > $1
		ORDER BY e.id
		LIMIT $2`

	q := postgres.GetQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, query, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("select pipeline events: %w", err)
	}
	defer rows.Close()

	var events []domain.PipelineEvent
	for rows.Next() {
		var (
			event  domain.PipelineEvent
			status string
		)
		if err = rows.Scan(&event.ID, &event.EntityKind, &event.EntityID, &status,
			&event.Service, &event.Message, &event.Payload, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pipeline event: %w", err)
		}
		event.Status = domain.PipelineStatus(status)
		events = append(events, event)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pipeline events: %w", err)
	}
	return events, nil
}

// LatestEventID — id последнего события журнала (0, если журнал пуст).
// Нужен, чтобы live-панель начинала с недавних событий, а не со всей истории.
func (r *PipelineRepo) LatestEventID(ctx context.Context) (int64, error) {
	const query = `SELECT coalesce(max(id), 0) FROM pipeline_events`

	var id int64
	q := postgres.GetQuerier(ctx, r.pool)
	if err := q.QueryRow(ctx, query).Scan(&id); err != nil {
		return 0, fmt.Errorf("select latest event id: %w", err)
	}
	return id, nil
}
