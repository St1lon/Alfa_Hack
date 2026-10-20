// Package apihttp — HTTP-контур сервиса go-api (BFF для фронта).
//
// СКЕЛЕТ. Заведён Go #1, чтобы фронт и Java писались параллельно с первой минуты;
// содержимое наполняет Go #2 (Client Context Engine, ранжирование, лента).
// Эндпоинты-заглушки отвечают 501 NOT_IMPLEMENTED — так видно, что контур жив,
// но данных ещё нет.
package apihttp

import (
	"context"
	"log/slog"
	"net/http"

	httpx "github.com/st1lon/alfa-hack/internal/transport/http"
)

// Pinger — порт проверки живости БД.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps — зависимости HTTP-контура go-api.
type Deps struct {
	DB  Pinger
	Log *slog.Logger
}

// Handlers реализует эндпоинты go-api.
type Handlers struct {
	deps Deps
}

type healthResponse struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type notImplementedResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
}

// NewHandlers создаёт обработчики go-api.
func NewHandlers(deps Deps) *Handlers {
	return &Handlers{deps: deps}
}

// Healthz — liveness.
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

// NotImplemented — заглушка эндпоинта, который реализует Go #2.
func (h *Handlers) NotImplemented(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, h.deps.Log, http.StatusNotImplemented, notImplementedResponse{
		Code:        "NOT_IMPLEMENTED",
		Description: "эндпоинт зарезервирован контрактом, реализация в работе",
		Owner:       "Go #2",
	})
}
