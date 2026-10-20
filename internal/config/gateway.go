package config

import (
	"time"

	"github.com/go-playground/validator/v10"
)

// Gateway — конфигурация llm-gateway. Ключ провайдера задаётся только здесь.
type Gateway struct {
	HTTP     HTTPConfig     `envPrefix:"GATEWAY_"`
	PG       PostgresConfig `envPrefix:"ALFA_"`
	Provider ProviderConfig `envPrefix:"LLM_"`
	Budget   BudgetConfig   `envPrefix:"LLM_"`
	// ModelsFile — файл с allowlist моделей, маршрутизацией и ценами.
	ModelsFile string `env:"LLM_MODELS_FILE" envDefault:"configs/llm.yaml"`
	// AuthToken — внутренний токен сервисов команды. Пустой = проверка выключена
	// (локальная разработка); на внешнем сервере обязателен.
	AuthToken string `env:"GATEWAY_AUTH_TOKEN"`
	LogLevel  string `env:"GATEWAY_LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
}

// ProviderConfig — доступ к провайдеру моделей хакатона.
// BaseURL и APIKey не помечены required: без них сервис поднимается и честно
// отвечает ошибкой на вызовы — иначе `docker compose up` ломается до выдачи ключа.
type ProviderConfig struct {
	BaseURL      string        `env:"UPSTREAM_BASE_URL"`
	APIKey       string        `env:"API_KEY"`
	Timeout      time.Duration `env:"TIMEOUT"       envDefault:"90s"`
	MaxRetries   int           `env:"MAX_RETRIES"   envDefault:"2" validate:"min=0"`
	RetryBackoff time.Duration `env:"RETRY_BACKOFF" envDefault:"2s"`
}

// BudgetConfig — дневные лимиты расхода на модели, в копейках.
// Ограничение 04 кейса: «общий ключ доступа к языковым моделям с дневным
// лимитом на количество токенов, ограничение в денежном эквиваленте — 500р».
// Soft-лимит на 80% от дневного потолка — сигнал экономить, не стоп.
type BudgetConfig struct {
	SoftLimitKop int64 `env:"SOFT_LIMIT_KOP" envDefault:"40000"`
	HardLimitKop int64 `env:"HARD_LIMIT_KOP" envDefault:"50000"`
}

// LoadGateway читает конфигурацию llm-gateway.
func LoadGateway(validate *validator.Validate) (*Gateway, error) {
	var cfg Gateway
	if err := parse(&cfg, validate); err != nil {
		return nil, err
	}
	return &cfg, nil
}
