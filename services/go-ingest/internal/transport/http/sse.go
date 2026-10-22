package ingesthttp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
)

const (
	// ssePollInterval — как часто стрим смотрит в журнал событий.
	ssePollInterval = time.Second
	// sseBatchLimit — сколько событий отдаём за один проход.
	sseBatchLimit = 100
	// sseHeartbeat — комментарий-пульс, чтобы прокси не рвали простаивающее соединение.
	sseHeartbeat = 15 * time.Second
	// sseDefaultBacklog — сколько последних событий показать новому подписчику,
	// который не назвал позицию: экран не пустой, но и тысяча старых событий не льётся.
	sseDefaultBacklog = 30
)

// sseEvent — событие пайплайна в проводном виде.
type sseEvent struct {
	ID         int64           `json:"id"`
	EntityKind string          `json:"entity_kind"`
	EntityID   string          `json:"entity_id"`
	Status     string          `json:"status,omitempty"`
	Service    string          `json:"service"`
	Message    string          `json:"message,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// StreamPipeline отдаёт события пайплайна по Server-Sent Events:
// GET /v1/pipeline/stream. Докрутка с места обрыва — через Last-Event-ID.
func (h *Handlers) StreamPipeline(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpWriteStreamUnsupported(w)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Отключаем буферизацию у обратных прокси — иначе события копятся и демо «залипает».
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	lastID, err := h.startPosition(r)
	if err != nil {
		h.deps.Log.ErrorContext(r.Context(), "resolve stream position failed",
			slog.String("error", err.Error()))
	}
	ticker := time.NewTicker(ssePollInterval)
	defer ticker.Stop()
	heartbeat := time.NewTicker(sseHeartbeat)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-ticker.C:
			events, err := h.deps.Events.ListAfter(ctx, lastID, sseBatchLimit)
			if err != nil {
				if isClientGone(err) {
					return
				}
				h.deps.Log.ErrorContext(ctx, "read pipeline events failed",
					slog.String("error", err.Error()))
				continue
			}
			for _, event := range events {
				if writeErr := writeSSE(w, event); writeErr != nil {
					return
				}
				lastID = event.ID
			}
			if len(events) > 0 {
				flusher.Flush()
			}
		}
	}
}

// startPosition — с какого id отдавать события. Явная позиция (заголовок
// Last-Event-ID или ?after=, after=0 — вся история) берётся как есть; без неё —
// хвост из sseDefaultBacklog последних событий.
func (h *Handlers) startPosition(r *http.Request) (int64, error) {
	if id, ok := lastEventID(r); ok {
		return id, nil
	}
	latest, err := h.deps.Events.LatestEventID(r.Context())
	if err != nil {
		return 0, fmt.Errorf("latest event id: %w", err)
	}
	return max(latest-sseDefaultBacklog, 0), nil
}

// lastEventID читает явную позицию докрутки: заголовок Last-Event-ID или ?after=.
func lastEventID(r *http.Request) (int64, bool) {
	raw := r.Header.Get("Last-Event-ID")
	if raw == "" {
		raw = r.URL.Query().Get("after")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 0 {
		return 0, false
	}
	return id, true
}

func writeSSE(w http.ResponseWriter, event domain.PipelineEvent) error {
	payload, err := json.Marshal(sseEvent{
		ID:         event.ID,
		EntityKind: event.EntityKind,
		EntityID:   event.EntityID,
		Status:     event.Status.String(),
		Service:    event.Service,
		Message:    event.Message,
		Payload:    event.Payload,
		CreatedAt:  event.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("marshal sse event: %w", err)
	}

	if _, err = fmt.Fprintf(w, "id: %d\nevent: pipeline\ndata: %s\n\n", event.ID, payload); err != nil {
		return fmt.Errorf("write sse event: %w", err)
	}
	return nil
}

func httpWriteStreamUnsupported(w http.ResponseWriter) {
	http.Error(w, "streaming unsupported", http.StatusInternalServerError)
}
