package config

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"

	sharedconfig "github.com/st1lon/alfa-hack/internal/config"
)

// Config — конфигурация go-api (Client Context Engine, ранжирование, BFF).
// Блок заведён Go #1 вместе со скелетом сервиса; наполняет его Go #2.
type Config struct {
	HTTP     sharedconfig.HTTPConfig     `envPrefix:"API_"`
	PG       sharedconfig.PostgresConfig `envPrefix:"ALFA_"`
	Gateway  UpstreamConfig              `envPrefix:"API_GATEWAY_"`
	Agent    UpstreamConfig              `envPrefix:"API_AGENT_"`
	LogLevel string                      `env:"API_LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
}

// UpstreamConfig — адрес и таймаут соседнего сервиса.
type UpstreamConfig struct {
	BaseURL string        `env:"BASE_URL"`
	Timeout time.Duration `env:"TIMEOUT" envDefault:"30s"`
}

// Load читает конфигурацию go-api.
func Load(validate *validator.Validate) (*Config, error) {
	var cfg Config
	if err := sharedconfig.Parse(&cfg, validate); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &cfg, nil
}
