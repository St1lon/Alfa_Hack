// Package ingesthttp — HTTP-контур сервиса go-ingest: вброс новости, live-стрим
// статусов пайплайна, health- и metrics-эндпоинты.
package ingesthttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
	httpx "github.com/st1lon/alfa-hack/internal/transport/http"
	"github.com/st1lon/alfa-hack/internal/transport/http/apierrors"
	ingestnews "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/news/ingest"
)

// NewsIngestor — порт приёма новости (usecase ingestnews).
type NewsIngestor interface {
	Execute(ctx context.Context, req *ingestnews.Request) (*ingestnews.Result, error)
}

// EventReader — порт чтения журнала событий пайплайна.
type EventReader interface {
	ListAfter(ctx context.Context, afterID int64, limit int) ([]domain.PipelineEvent, error)
	LatestEventID(ctx context.Context) (int64, error)
}

// StatsReader — порт снимка воронки пайплайна.
type StatsReader interface {
	Snapshot(ctx context.Context) (*domain.PipelineStats, error)
}

// Pinger — порт проверки живости БД.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps — зависимости HTTP-контура go-ingest.
type Deps struct {
	Ingest NewsIngestor
	Events EventReader
	Stats  StatsReader
	DB     Pinger
	Log    *slog.Logger
}

// Handlers реализует эндпоинты go-ingest.
type Handlers struct {
	deps Deps
}

// ingestNewsRequest — вброс новости текстом или ссылкой (критерий 05, живое демо).
type ingestNewsRequest struct {
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	Publisher   string     `json:"publisher"`
	Lang        string     `json:"lang"`
	PublishedAt *time.Time `json:"published_at"`
}

// ingestNewsResponse — результат вброса.
type ingestNewsResponse struct {
	NewsID    string `json:"news_id,omitempty"`
	Duplicate bool   `json:"duplicate"`
	Status    string `json:"status"`
}

// healthResponse — ответ health- и readiness-проб.
type healthResponse struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// metricsResponse — счётчики пайплайна для внутреннего дашборда команды.
type metricsResponse struct {
	NewsByStatus    map[string]int `json:"news_by_status"`
	StoriesByStatus map[string]int `json:"stories_by_status"`
	ResearchDocs    int            `json:"research_documents"`
	ResearchChunks  int            `json:"research_chunks"`
	NewsLast24h     int            `json:"news_last_24h"`
	LastFetchedAt   *time.Time     `json:"last_fetched_at,omitempty"`
}

// NewHandlers создаёт обработчики go-ingest.
func NewHandlers(deps Deps) *Handlers {
	return &Handlers{deps: deps}
}

// IngestNews принимает новость вручную: POST /v1/news.
func (h *Handlers) IngestNews(w http.ResponseWriter, r *http.Request) {
	var body ingestNewsRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteJSON(w, h.deps.Log, http.StatusBadRequest, apierrors.BadRequest("malformed json body"))
		return
	}

	result, err := h.deps.Ingest.Execute(r.Context(), &ingestnews.Request{
		URL:         body.URL,
		Title:       body.Title,
		Body:        body.Body,
		Publisher:   body.Publisher,
		Lang:        body.Lang,
		PublishedAt: body.PublishedAt,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}

	if result.Duplicate {
		httpx.WriteJSON(w, h.deps.Log, http.StatusOK, ingestNewsResponse{
			Duplicate: true,
			Status:    "duplicate",
		})
		return
	}

	httpx.WriteJSON(w, h.deps.Log, http.StatusCreated, ingestNewsResponse{
		NewsID: result.NewsID.String(),
		Status: domain.StatusRaw.String(),
	})
}

// Healthz — liveness: процесс жив.
func (h *Handlers) Healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, h.deps.Log, http.StatusOK, healthResponse{Status: "ok"})
}

// Readyz — readiness: есть коннект к БД.
func (h *Handlers) Readyz(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.DB.Ping(r.Context()); err != nil {
		httpx.WriteJSON(w, h.deps.Log, http.StatusServiceUnavailable, healthResponse{
			Status: "unavailable",
			Detail: "database is not reachable",
		})
		return
	}
	httpx.WriteJSON(w, h.deps.Log, http.StatusOK, healthResponse{Status: "ok"})
}

// Metrics отдаёт счётчики пайплайна.
func (h *Handlers) Metrics(w http.ResponseWriter, r *http.Request) {
	stats, err := h.deps.Stats.Snapshot(r.Context())
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, h.deps.Log, http.StatusOK, metricsResponse{
		NewsByStatus:    statusMap(stats.NewsByStatus),
		StoriesByStatus: statusMap(stats.StoriesByStatus),
		ResearchDocs:    stats.ResearchDocs,
		ResearchChunks:  stats.ResearchChunks,
		NewsLast24h:     stats.NewsLast24h,
		LastFetchedAt:   stats.LastFetchedAt,
	})
}

// writeErr маппит ошибку usecase в HTTP-ответ и логирует серверные сбои.
func (h *Handlers) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	status, body := apierrors.Map(err)
	if status >= http.StatusInternalServerError {
		h.deps.Log.ErrorContext(r.Context(), "request failed",
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
	}
	httpx.WriteJSON(w, h.deps.Log, status, body)
}

// isClientGone отличает «клиент закрыл SSE-соединение» от настоящей ошибки.
func isClientGone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// statusMap переводит доменные статусы в ключи JSON.
func statusMap(counts map[domain.PipelineStatus]int) map[string]int {
	out := make(map[string]int, len(counts))
	for status, count := range counts {
		out[status.String()] = count
	}
	return out
}
