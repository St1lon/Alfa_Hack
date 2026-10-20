// Package gatewayapp — composition root сервиса llm-gateway.
package gatewayapp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/st1lon/alfa-hack/internal/config"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres/repository"
	"github.com/st1lon/alfa-hack/internal/infra/llmprovider"
	httpx "github.com/st1lon/alfa-hack/internal/transport/http"
	gatewayhttp "github.com/st1lon/alfa-hack/internal/transport/http/gateway"
	completellm "github.com/st1lon/alfa-hack/internal/usecase/llm/complete"
)

// App держит собранные ресурсы llm-gateway.
type App struct {
	cfg  *config.Gateway
	log  *slog.Logger
	pool *pgxpool.Pool
	http *httpx.Server
}

// Build собирает сервис: allowlist моделей, клиент провайдера, кэш, аудит, HTTP.
func Build(ctx context.Context, cfg *config.Gateway, log *slog.Logger) (*App, error) {
	models, err := config.LoadLLMModels(cfg.ModelsFile)
	if err != nil {
		return nil, fmt.Errorf("load llm allowlist: %w", err)
	}

	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{
		Host:     cfg.PG.Host,
		Port:     cfg.PG.Port,
		User:     cfg.PG.User,
		Password: cfg.PG.Password,
		Database: cfg.PG.Database,
		SSLMode:  cfg.PG.SSLMode,
		MaxConns: cfg.PG.MaxConns,
	})
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	if cfg.Provider.BaseURL == "" || cfg.Provider.APIKey == "" {
		// Сервис всё равно поднимается: остальные стеки пишутся против живого
		// контракта, а вызовы честно отваливаются как UPSTREAM_FAILED.
		log.Warn("llm provider is not configured; /v1/complete will fail until LLM_UPSTREAM_BASE_URL and LLM_API_KEY are set")
	}

	llm := repository.NewLLMRepo(pool)
	provider := llmprovider.New(llmprovider.Config{
		BaseURL:      cfg.Provider.BaseURL,
		APIKey:       cfg.Provider.APIKey,
		Timeout:      cfg.Provider.Timeout,
		MaxRetries:   cfg.Provider.MaxRetries,
		RetryBackoff: cfg.Provider.RetryBackoff,
	})

	registry := completellm.NewRegistry(models.DomainModels(), models.DomainRouting(), models.FallbackModel)
	limits := completellm.Limits{
		SoftLimitKop: cfg.Budget.SoftLimitKop,
		HardLimitKop: cfg.Budget.HardLimitKop,
	}

	complete := completellm.NewUsecase(registry, provider, llm, llm, llm, limits)

	handlers := gatewayhttp.NewHandlers(gatewayhttp.Deps{
		Complete: complete,
		Registry: registry,
		Budget:   llm,
		DB:       llm,
		Limits:   limits,
		Log:      log,
	})

	server := httpx.NewServer(httpx.ServerConfig{
		Host:              cfg.HTTP.Host,
		Port:              cfg.HTTP.Port,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}, gatewayhttp.NewRouter(handlers, cfg.AuthToken))

	return &App{cfg: cfg, log: log, pool: pool, http: server}, nil
}

// Run запускает HTTP-сервер (блокирующе).
func (a *App) Run() error {
	a.log.Info("starting llm-gateway",
		slog.String("host", a.cfg.HTTP.Host),
		slog.Int("port", a.cfg.HTTP.Port),
		slog.Int64("hard_limit_kop", a.cfg.Budget.HardLimitKop),
	)
	if err := a.http.Run(); err != nil {
		return fmt.Errorf("run http server: %w", err)
	}
	return nil
}

// Shutdown корректно освобождает ресурсы.
func (a *App) Shutdown(ctx context.Context) {
	if err := a.http.Shutdown(ctx); err != nil {
		a.log.Error("http shutdown failed", slog.String("error", err.Error()))
	}
	a.pool.Close()
	a.log.Info("llm-gateway stopped")
}
