// Command research-loader — однократная загрузка аналитики ЦМИ Альфа-Банка:
// OCR PDF-сканов → чанки с номером страницы → research_chunks (задача G1-13).
// Повторный запуск пропускает уже загруженные документы; RESEARCH_FORCE=true
// перезаливает всё.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/st1lon/alfa-hack/internal/app"
	ingestapp "github.com/st1lon/alfa-hack/services/go-ingest/internal/app"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/config"
	loadresearch "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/research/load"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadResearchLoader(validator.New())
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	log := app.NewLogger(cfg.LogLevel)
	started := time.Now()
	log.Info("research loader started",
		slog.String("dir", cfg.Research.Dir),
		slog.Int("ocr_workers", cfg.Research.OCRWorkers),
		slog.Bool("force", cfg.Research.Force),
	)

	result, err := ingestapp.RunResearchLoader(ctx, cfg, log)
	if result != nil {
		report(log, result)
	}
	if err != nil {
		log.Error("research loader failed", slog.String("error", err.Error()))
		return 1
	}

	failed := result.Count(loadresearch.OutcomeFailed)
	log.Info("research loader finished",
		slog.Int("loaded", result.Count(loadresearch.OutcomeLoaded)),
		slog.Int("skipped", result.Count(loadresearch.OutcomeSkipped)),
		slog.Int("failed", failed),
		slog.Int64("duration_s", int64(time.Since(started).Seconds())),
	)
	if failed > 0 {
		// Ненулевой код, чтобы compose и CI показали проблему, а не молча
		// оставили агента без позиции банка. Чаще всего это `make data` не запущен.
		return 1
	}
	return 0
}

func report(log *slog.Logger, result *loadresearch.Result) {
	for _, doc := range result.Documents {
		attrs := []any{
			slog.String("code", doc.Code),
			slog.String("outcome", string(doc.Outcome)),
			slog.Int("pages", doc.Pages),
			slog.Int("chunks", doc.Chunks),
		}
		if doc.Outcome == loadresearch.OutcomeFailed {
			log.Error("research document failed", append(attrs, slog.String("error", doc.Error))...)
			continue
		}
		log.Info("research document", attrs...)
	}
}
