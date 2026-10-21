// Package stagetest — рукописные заглушки портов для тестов этапов
// оркестратора (CLAUDE.md §10: для простых случаев допустимы стабы).
package stagetest

import (
	"context"
	"sync"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// Tx — менеджер транзакций, который просто выполняет функцию.
type Tx struct{ Calls int }

// Do выполняет fn без транзакции.
func (t *Tx) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	t.Calls++
	return fn(ctx)
}

// Attempts — учёт попыток в памяти. DeadAfter — с какой попытки запись «умирает».
type Attempts struct {
	mu        sync.Mutex
	DeadAfter int
	Counts    map[string]int
	Cleared   []string
}

// NewAttempts создаёт учёт попыток.
func NewAttempts(deadAfter int) *Attempts {
	return &Attempts{DeadAfter: deadAfter, Counts: map[string]int{}}
}

// Fail записывает попытку.
func (a *Attempts) Fail(_ context.Context, kind, id, stage string, _ error) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := kind + "/" + id + "/" + stage
	a.Counts[key]++
	return a.DeadAfter > 0 && a.Counts[key] >= a.DeadAfter, nil
}

// Clear запоминает очистку.
func (a *Attempts) Clear(_ context.Context, kind, id, stage string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Cleared = append(a.Cleared, kind+"/"+id+"/"+stage)
	return nil
}

// Total — сколько всего неудачных попыток записано.
func (a *Attempts) Total() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	total := 0
	for _, n := range a.Counts {
		total += n
	}
	return total
}

// Event — записанное событие пайплайна.
type Event struct {
	Kind, ID string
	Status   domain.PipelineStatus
	Message  string
}

// Events — журнал событий в памяти.
type Events struct {
	mu   sync.Mutex
	List []Event
}

// Append записывает событие.
func (e *Events) Append(
	_ context.Context, kind, id string, status domain.PipelineStatus, _, message string, _ any,
) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.List = append(e.List, Event{Kind: kind, ID: id, Status: status, Message: message})
	return nil
}
