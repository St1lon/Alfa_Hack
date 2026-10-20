// Command llm-gateway — единственный выход к языковым моделям: allowlist,
// маршрутизация, кэш, дневной бюджет и аудит вызовов.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-playground/validator/v10"

	"github.com/st1lon/alfa-hack/internal/app"
	gatewayapp "github.com/st1lon/alfa-hack/internal/app/gateway"
	"github.com/st1lon/alfa-hack/internal/config"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadGateway(validator.New())
	if err != nil {
		_, _ = os.Stderr.WriteString("load config: " + err.Error() + "\n")
		return 1
	}

	log := app.NewLogger(cfg.LogLevel)

	application, err := gatewayapp.Build(ctx, cfg, log)
	if err != nil {
		log.Error("failed to build llm-gateway", "error", err.Error())
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
			log.Error("llm-gateway error", "error", err.Error())
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	application.Shutdown(shutdownCtx)

	return 0
}
