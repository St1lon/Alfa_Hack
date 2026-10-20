package domain

import "time"

// PipelineEvent — событие продвижения сущности по пайплайну.
// Источник live-панели на защите (критерии 04 и 11).
type PipelineEvent struct {
	ID         int64
	EntityKind string
	EntityID   string
	Status     PipelineStatus
	Service    string
	Message    string
	Payload    []byte
	CreatedAt  time.Time
}
