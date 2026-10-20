package gatewayhttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/st1lon/alfa-hack/internal/transport/http/middleware"
)

// NewRouter собирает роутер llm-gateway. CORS не нужен: сюда ходят только
// сервисы команды, браузер — никогда. Внутренний токен проверяется на /v1.
func NewRouter(handlers *Handlers, serviceToken string) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestLogger(handlers.deps.Log))

	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz)

	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(middleware.ServiceToken(serviceToken))
		v1.Post("/complete", handlers.Complete)
		v1.Get("/models", handlers.Models)
		v1.Get("/budget", handlers.Budget)
	})

	return r
}
