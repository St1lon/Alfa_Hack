// Package apiapp — composition root сервиса go-api.
//
// СКЕЛЕТ Go #1: поднимает HTTP и пул БД, остальное собирает Go #2 здесь же.
package apiapp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
	httpx "github.com/st1lon/alfa-hack/internal/transport/http"
	"github.com/st1lon/alfa-hack/services/go-api/internal/config"
	apihttp "github.com/st1lon/alfa-hack/services/go-api/internal/transport/http"
)

// App держит собранные ресурсы go-api.
type App struct {
	cfg  *config.Config
	log  *slog.Logger
	pool *pgxpool.Pool
	http *httpx.Server
}

// Build собирает сервис go-api.
func Build(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
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

	handlers := apihttp.NewHandlers(apihttp.Deps{
		DB:  postgres.NewHealth(pool),
		Log: log,
	})

	server := httpx.NewServer(httpx.ServerConfig{
		Host:              cfg.HTTP.Host,
		Port:              cfg.HTTP.Port,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}, apihttp.NewRouter(handlers))

	return &App{cfg: cfg, log: log, pool: pool, http: server}, nil
}

// Run запускает HTTP-сервер (блокирующе).
func (a *App) Run() error {
	a.log.Info("starting go-api",
		slog.String("host", a.cfg.HTTP.Host),
		slog.Int("port", a.cfg.HTTP.Port),
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
	a.log.Info("go-api stopped")
}
