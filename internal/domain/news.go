package domain

import (
	"time"

	"github.com/google/uuid"
)

// SourceKind — тип внешнего источника новостей.
type SourceKind string

// Поддерживаемые типы источников.
const (
	SourceKindRSS  SourceKind = "rss"
	SourceKindHTML SourceKind = "html"
)

// NewsSource — внешняя лента, которую опрашивает парсер.
type NewsSource struct {
	ID           int16
	Code         string
	Title        string
	Kind         SourceKind
	URL          string
	Enabled      bool
	PollInterval time.Duration
	LastPolledAt *time.Time
}

// News — одна новость из внешнего источника либо вброшенная вручную на демо.
type News struct {
	ID           uuid.UUID
	SourceID     *int16
	URL          string
	CanonicalURL string
	URLHash      string // sha256 канонизированного URL; пусто для вброса текстом
	TextHash     string // sha256 нормализованного текста — основа дедупа
	Title        string
	Body         string
	Publisher    string
	Lang         string
	PublishedAt  *time.Time
	FetchedAt    time.Time
	Status       PipelineStatus
	StoryID      *uuid.UUID
}

// Story — сюжет: перепечатки одной новости, склеенные в одну единицу анализа.
// Дорогая модель вызывается на сюжет, а не на каждую перепечатку.
type Story struct {
	ID               uuid.UUID
	Title            string
	Summary          string
	FirstPublishedAt *time.Time
	LastPublishedAt  *time.Time
	SourcesCount     int
	Status           PipelineStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
