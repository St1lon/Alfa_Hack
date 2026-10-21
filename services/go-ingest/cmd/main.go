// Command go-ingest — парсер новостей, загрузчик аналитики, оркестратор и SSE.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-playground/validator/v10"

	"github.com/st1lon/alfa-hack/internal/app"
	ingestapp "github.com/st1lon/alfa-hack/services/go-ingest/internal/app"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/config"
)

func main() {
	// run() держит defer'ы в своей области: os.Exit рядом с defer их бы пропустил.
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(validator.New())
	if err != nil {
		// Логгер ещё не собран: конфиг — его источник уровня логирования.
		_, _ = os.Stderr.WriteString("load config: " + err.Error() + "\n")
		return 1
	}

	log := app.NewLogger(cfg.LogLevel)

	application, err := ingestapp.Build(ctx, cfg, log)
	if err != nil {
		log.Error("failed to build go-ingest", "error", err.Error())
		return 1
	}

	runErr := make(chan error, 1)
	go func() {
		runErr <- application.Run(ctx)
	}()

	select {
	case <-ctx.Done():
	case err = <-runErr:
		if err != nil {
			log.Error("go-ingest error", "error", err.Error())
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	application.Shutdown(shutdownCtx)

	return 0
}
