// Package orchestrator — воркер пайплайна: по тику прогоняет этапы
// RAW → DEDUPED → ENRICHED → сюжет → SCORED → ANALYZED. Driving-адаптер:
// решения о переходах принимают usecase'ы этапов, воркер только крутит их.
package orchestrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/stage"
)

// Config — параметры воркера.
type Config struct {
	Tick time.Duration
	// MaxBatchesPerTick — сколько пачек этап может прогнать за тик, если работа
	// есть. Ограничение не даёт одному этапу с большим хвостом занять весь тик.
	MaxBatchesPerTick int
}

// Stage — один этап пайплайна в порядке воронки.
type Stage struct {
	Name  string
	Batch int
	Run   func(ctx context.Context, req *stage.Request) (*stage.Result, error)
}

// Worker прогоняет этапы по тику.
type Worker struct {
	cfg    Config
	stages []Stage
	log    *slog.Logger
	// waiting — какие этапы сейчас ждут соседний сервис; логируем только смену
	// состояния, иначе неготовый go-api заспамил бы лог на каждом тике.
	waiting map[string]bool
}

// New создаёт воркер пайплайна.
func New(cfg Config, stages []Stage, log *slog.Logger) *Worker {
	return &Worker{cfg: cfg, stages: stages, log: log, waiting: make(map[string]bool)}
}

// Run крутит пайплайн до отмены контекста. Первый тик — сразу на старте.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.Tick)
	defer ticker.Stop()

	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			w.log.Info("orchestrator stopped")
			return nil
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick прогоняет все этапы по порядку: то, что один этап продвинул, следующий
// подхватит в этом же тике — новость доходит до агента без лишнего ожидания.
func (w *Worker) tick(ctx context.Context) {
	for _, s := range w.stages {
		if ctx.Err() != nil {
			return
		}
		w.runStage(ctx, s)
	}
}

func (w *Worker) runStage(ctx context.Context, s Stage) {
	var total stage.Result
	for range w.cfg.MaxBatchesPerTick {
		result, err := s.Run(ctx, &stage.Request{Limit: s.Batch})
		if err != nil {
			w.log.Error("pipeline stage failed", slog.String("stage", s.Name), slog.String("error", err.Error()))
			return
		}
		total.Processed += result.Processed
		total.Filtered += result.Filtered
		total.Failed += result.Failed
		total.Dead += result.Dead
		total.NotReady = result.NotReady
		if result.NotReady || result.Touched() < s.Batch {
			break
		}
	}

	w.reportWaiting(s.Name, total.NotReady)
	if total.Touched() > 0 {
		w.log.Info("pipeline stage",
			slog.String("stage", s.Name),
			slog.Int("processed", total.Processed),
			slog.Int("filtered", total.Filtered),
			slog.Int("failed", total.Failed),
			slog.Int("dead", total.Dead),
		)
	}
}

func (w *Worker) reportWaiting(name string, waiting bool) {
	if w.waiting[name] == waiting {
		return
	}
	w.waiting[name] = waiting
	if waiting {
		w.log.Info("pipeline stage waits for downstream service", slog.String("stage", name))
		return
	}
	w.log.Info("pipeline stage downstream is ready", slog.String("stage", name))
}
