// Package ingestapp — composition root сервиса go-ingest: ручная сборка
// зависимостей и управление жизненным циклом (CLAUDE.md §2).
package ingestapp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/st1lon/alfa-hack/internal/config"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
	"github.com/st1lon/alfa-hack/internal/infra/db/postgres/repository"
	"github.com/st1lon/alfa-hack/internal/infra/newsfeed"
	httpx "github.com/st1lon/alfa-hack/internal/transport/http"
	ingesthttp "github.com/st1lon/alfa-hack/internal/transport/http/ingest"
	ingestnews "github.com/st1lon/alfa-hack/internal/usecase/news/ingest"
	"github.com/st1lon/alfa-hack/internal/worker/newspoller"
)

// App держит собранные ресурсы go-ingest.
type App struct {
	cfg    *config.Ingest
	log    *slog.Logger
	pool   *pgxpool.Pool
	http   *httpx.Server
	poller *newspoller.Worker
}

// Build собирает сервис: пул БД, репозитории, usecase, воркер лент, HTTP.
func Build(ctx context.Context, cfg *config.Ingest, log *slog.Logger) (*App, error) {
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

	news := repository.NewNewsRepo(pool)
	events := repository.NewPipelineRepo(pool)

	ingestUsecase := ingestnews.NewUsecase(news, events)

	handlers := ingesthttp.NewHandlers(ingesthttp.Deps{
		Ingest:  ingestUsecase,
		Events:  events,
		Counter: news,
		DB:      news,
		Log:     log,
	})

	var poller *newspoller.Worker
	if cfg.Poller.Enabled {
		fetcher := newsfeed.NewFetcher(cfg.Poller.FetchTimeout, cfg.Poller.UserAgent)
		poller = newspoller.New(
			newspoller.Config{
				Tick:              cfg.Poller.Tick,
				MaxItemsPerPoll:   cfg.Poller.MaxItemsPerPoll,
				HydrateMinBodyLen: cfg.Poller.HydrateMinBodyLen,
				MaxHydratePerPoll: cfg.Poller.MaxHydratePerPoll,
			},
			news, fetcher, ingestUsecase, log,
		)
	}

	server := httpx.NewServer(httpx.ServerConfig{
		Host:              cfg.HTTP.Host,
		Port:              cfg.HTTP.Port,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}, ingesthttp.NewRouter(handlers))

	return &App{cfg: cfg, log: log, pool: pool, http: server, poller: poller}, nil
}

// Run запускает HTTP-сервер и воркер опроса лент до отмены контекста.
func (a *App) Run(ctx context.Context) error {
	a.log.Info("starting go-ingest",
		slog.String("host", a.cfg.HTTP.Host),
		slog.Int("port", a.cfg.HTTP.Port),
		slog.Bool("poller_enabled", a.cfg.Poller.Enabled),
	)

	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		if err := a.http.Run(); err != nil {
			return fmt.Errorf("run http server: %w", err)
		}
		return nil
	})

	if a.poller != nil {
		group.Go(func() error {
			return a.poller.Run(groupCtx)
		})
	}

	if err := group.Wait(); err != nil {
		return fmt.Errorf("go-ingest stopped with error: %w", err)
	}
	return nil
}

// Shutdown корректно освобождает ресурсы.
func (a *App) Shutdown(ctx context.Context) {
	if err := a.http.Shutdown(ctx); err != nil {
		a.log.Error("http shutdown failed", slog.String("error", err.Error()))
	}
	a.pool.Close()
	a.log.Info("go-ingest stopped")
}
