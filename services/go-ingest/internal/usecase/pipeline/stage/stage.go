// Package stage — общее для этапов оркестратора: результат этапа, порты
// учёта попыток и журнала событий, обработка сбоя записи.
package stage

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// ServiceName — кто пишет события пайплайна.
const ServiceName = "go-ingest"

// Виды сущностей в job_attempts и pipeline_events.
const (
	KindNews  = "news"
	KindStory = "story"
)

// Result — итог одного прогона этапа на пачке записей.
type Result struct {
	Processed int // успешно перешли в следующий статус
	Filtered  int // штатно отсеяны (шум, нерелевантный сюжет)
	Failed    int // сбой, отложены на ретрай
	Dead      int // попытки кончились, ушли в FAILED и dead_letters
	// NotReady — соседний сервис ещё не готов; этап остановлен до следующего тика.
	NotReady bool
}

// Touched — сколько записей этап реально обработал (для решения «гнать ещё пачку»).
func (r *Result) Touched() int {
	return r.Processed + r.Filtered + r.Failed + r.Dead
}

// Request — размер пачки, общий для всех этапов.
type Request struct {
	Limit int
}

// Validate проверяет размер пачки.
func (r *Request) Validate() error {
	if r.Limit <= 0 {
		return fmt.Errorf("%w: batch limit must be positive", domain.ErrValidation)
	}
	return nil
}

// Attempts — порт учёта неудачных попыток (job_attempts, dead_letters).
type Attempts interface {
	Fail(ctx context.Context, kind, id, stage string, cause error) (bool, error)
	Clear(ctx context.Context, kind, id, stage string) error
}

// EventSink — порт журнала событий пайплайна (источник SSE-стрима).
type EventSink interface {
	Append(
		ctx context.Context,
		entityKind, entityID string,
		status domain.PipelineStatus,
		service, message string,
		payload any,
	) error
}

// Failure — всё, что нужно, чтобы обработать сбой одной записи.
type Failure struct {
	Kind, Stage string
	ID          uuid.UUID
	Cause       error
	// MarkFailed переводит запись в FAILED, когда попытки кончились.
	MarkFailed func(ctx context.Context) error
}

// RecordFailure записывает попытку; если попыток больше нет — переводит
// запись в FAILED и пишет событие. Ошибка — только ошибка самой базы.
func RecordFailure(ctx context.Context, attempts Attempts, events EventSink, f Failure, result *Result) error {
	dead, err := attempts.Fail(ctx, f.Kind, f.ID.String(), f.Stage, f.Cause)
	if err != nil {
		return err
	}
	if !dead {
		result.Failed++
		return nil
	}
	if err = f.MarkFailed(ctx); err != nil {
		return err
	}
	result.Dead++
	Emit(ctx, events, f.Kind, f.ID, domain.StatusFailed,
		"этап "+f.Stage+" не прошёл после всех попыток", map[string]any{"error": f.Cause.Error()})
	return nil
}

// Emit пишет событие для live-панели. Сбой журнала не отменяет переход:
// журнал — витрина, а не источник истины.
func Emit(
	ctx context.Context, events EventSink, kind string, id uuid.UUID,
	status domain.PipelineStatus, message string, payload any,
) {
	if events == nil {
		return
	}
	_ = events.Append(ctx, kind, id.String(), status, ServiceName, message, payload)
}
