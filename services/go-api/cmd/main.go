// Command go-api — Client Context Engine, ранжирование и BFF для фронта.
// Скелет заведён Go #1; содержимое — зона Go #2.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-playground/validator/v10"

	"github.com/st1lon/alfa-hack/internal/app"
	apiapp "github.com/st1lon/alfa-hack/services/go-api/internal/app"
	"github.com/st1lon/alfa-hack/services/go-api/internal/config"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(validator.New())
	if err != nil {
		_, _ = os.Stderr.WriteString("load config: " + err.Error() + "\n")
		return 1
	}

	log := app.NewLogger(cfg.LogLevel)

	application, err := apiapp.Build(ctx, cfg, log)
	if err != nil {
		log.Error("failed to build go-api", "error", err.Error())
		return 1
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- application.Run()
	}()

	select {
	case <-ctx.Done():
	case err = <-serverErr:
		if err != nil {
			log.Error("go-api error", "error", err.Error())
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	application.Shutdown(shutdownCtx)

	return 0
}
