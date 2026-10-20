package apihttp

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

// NewRouter собирает роутер go-api. Набор путей — это заявленный контракт BFF
// (TEAM_PLAN §6, P0 п.4); тела отдаёт Go #2.
func NewRouter(handlers *Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           int(corsMaxAge.Seconds()),
	}))
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestLogger(handlers.deps.Log))

	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz)

	r.Route("/v1", func(v1 chi.Router) {
		// Лента инсайтов с фильтрами.
		v1.Get("/feed", handlers.NotImplemented)
		// Карточка разбора.
		v1.Get("/insights/{insightID}", handlers.NotImplemented)
		// Уточняющий вопрос по инсайту — прокси в java-agent.
		v1.Post("/insights/{insightID}/questions", handlers.NotImplemented)
		// Оценка инсайта с тегами причин.
		v1.Post("/insights/{insightID}/feedback", handlers.NotImplemented)
		// Список компаний для переключателя.
		v1.Get("/companies", handlers.NotImplemented)
		// Профиль клиента: что система поняла про компанию и откуда.
		v1.Get("/companies/{companyID}", handlers.NotImplemented)
		// Переопределения выведенных признаков.
		v1.Put("/companies/{companyID}/overrides", handlers.NotImplemented)
	})

	return r
}
