package config

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"

	sharedconfig "github.com/st1lon/alfa-hack/internal/config"
)

// Config — конфигурация сервиса go-ingest (парсер, оркестратор, SSE).
type Config struct {
	HTTP     sharedconfig.HTTPConfig     `envPrefix:"INGEST_"`
	PG       sharedconfig.PostgresConfig `envPrefix:"ALFA_"`
	Poller   PollerConfig                `envPrefix:"INGEST_"`
	LogLevel string                      `env:"INGEST_LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
}

// PollerConfig — параметры опроса внешних лент.
type PollerConfig struct {
	Enabled bool `env:"POLLER_ENABLED" envDefault:"true"`
	// Tick — как часто воркер смотрит, какие ленты пора опросить. Сам интервал
	// опроса хранится у источника в БД (news_sources.poll_interval_sec).
	Tick            time.Duration `env:"POLLER_TICK"          envDefault:"60s"`
	MaxItemsPerPoll int           `env:"POLLER_MAX_ITEMS"     envDefault:"40" validate:"min=1"`
	FetchTimeout    time.Duration `env:"POLLER_FETCH_TIMEOUT" envDefault:"25s"`
	// HydrateMinBodyLen — порог, ниже которого текст из ленты считаем анонсом
	// и идём за полным текстом на страницу первоисточника.
	HydrateMinBodyLen int    `env:"POLLER_HYDRATE_MIN_BODY_LEN" envDefault:"300" validate:"min=0"`
	MaxHydratePerPoll int    `env:"POLLER_MAX_HYDRATE"          envDefault:"15" validate:"min=0"`
	UserAgent         string `env:"POLLER_USER_AGENT"    envDefault:"AlfaHackBot/0.1 (hackathon prototype)"`
}

// Load читает конфигурацию go-ingest.
func Load(validate *validator.Validate) (*Config, error) {
	var cfg Config
	if err := sharedconfig.Parse(&cfg, validate); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &cfg, nil
}
