package config

import (
	"time"

	"github.com/go-playground/validator/v10"
)

// API — конфигурация go-api (Client Context Engine, ранжирование, BFF).
// Блок заведён Go #1 вместе со скелетом сервиса; наполняет его Go #2.
type API struct {
	HTTP     HTTPConfig     `envPrefix:"API_"`
	PG       PostgresConfig `envPrefix:"ALFA_"`
	Gateway  UpstreamConfig `envPrefix:"API_GATEWAY_"`
	Agent    UpstreamConfig `envPrefix:"API_AGENT_"`
	LogLevel string         `env:"API_LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
}

// UpstreamConfig — адрес и таймаут соседнего сервиса.
type UpstreamConfig struct {
	BaseURL string        `env:"BASE_URL"`
	Timeout time.Duration `env:"TIMEOUT" envDefault:"30s"`
}

// LoadAPI читает конфигурацию go-api.
func LoadAPI(validate *validator.Validate) (*API, error) {
	var cfg API
	if err := parse(&cfg, validate); err != nil {
		return nil, err
	}
	return &cfg, nil
}
