package ingesthttp

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/st1lon/alfa-hack/internal/transport/http/middleware"
)

// corsMaxAge — на сколько браузер кэширует ответ на preflight OPTIONS.
const corsMaxAge = 300 * time.Second

// NewRouter собирает роутер go-ingest. CORS открыт: фронт Go #2 подписывается
// на SSE напрямую из браузера, авторизации пользователей в прототипе нет.
func NewRouter(handlers *Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "Last-Event-ID"},
		AllowCredentials: false,
		MaxAge:           int(corsMaxAge.Seconds()),
	}))
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestLogger(handlers.deps.Log))

	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz)
	r.Get("/metrics", handlers.Metrics)
	r.Get("/panel", handlers.Panel)

	r.Route("/v1", func(v1 chi.Router) {
		v1.Post("/news", handlers.IngestNews)
		v1.Get("/pipeline/stream", handlers.StreamPipeline)
	})

	return r
}
