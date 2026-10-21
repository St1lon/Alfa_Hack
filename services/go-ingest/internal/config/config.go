package config

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"

	sharedconfig "github.com/st1lon/alfa-hack/internal/config"
)

// Config — конфигурация сервиса go-ingest (парсер, оркестратор, SSE).
type Config struct {
	HTTP         sharedconfig.HTTPConfig     `envPrefix:"INGEST_"`
	PG           sharedconfig.PostgresConfig `envPrefix:"ALFA_"`
	Poller       PollerConfig                `envPrefix:"INGEST_"`
	Orchestrator OrchestratorConfig          `envPrefix:"INGEST_ORCH_"`
	Downstream   DownstreamConfig            `envPrefix:"INGEST_"`
	// UseMLFallback — обогащать новости словарями внутри Go, а не через py-ml.
	// Имя переменной — из TEAM_PLAN §5: штатный режим первых дней.
	UseMLFallback bool   `env:"USE_ML_FALLBACK"  envDefault:"true"`
	LogLevel      string `env:"INGEST_LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
}

// OrchestratorConfig — параметры воркера пайплайна (G1-14).
type OrchestratorConfig struct {
	Enabled bool          `env:"ENABLED" envDefault:"true"`
	Tick    time.Duration `env:"TICK"    envDefault:"10s"`
	Batch   int           `env:"BATCH"   envDefault:"50" validate:"min=1"`
	// AnalyzeBatch маленький: разбор агентом — десятки секунд на пару,
	// а сюжеты держатся захваченными до конца пачки.
	AnalyzeBatch      int `env:"ANALYZE_BATCH"        envDefault:"2"  validate:"min=1"`
	MaxBatchesPerTick int `env:"MAX_BATCHES_PER_TICK" envDefault:"10" validate:"min=1"`

	MaxAttempts int           `env:"MAX_ATTEMPTS" envDefault:"5" validate:"min=1"`
	BackoffBase time.Duration `env:"BACKOFF_BASE" envDefault:"30s"`
	BackoffMax  time.Duration `env:"BACKOFF_MAX"  envDefault:"30m"`

	NoiseThreshold       float64       `env:"NOISE_THRESHOLD"       envDefault:"0.8" validate:"gte=0,lte=1"`
	ClusterSimilarity    float64       `env:"CLUSTER_SIMILARITY"    envDefault:"0.6" validate:"gt=0,lte=1"`
	ClusterWindow        time.Duration `env:"CLUSTER_WINDOW"        envDefault:"72h"`
	RelevanceThreshold   float64       `env:"RELEVANCE_THRESHOLD"   envDefault:"0.5" validate:"gte=0,lte=1"`
	MaxCompaniesPerStory int           `env:"MAX_COMPANIES_PER_STORY" envDefault:"3" validate:"min=1"`
	EnrichmentDictionary string        `env:"ENRICHMENT_DICTIONARY" envDefault:"configs/enrichment.yaml"`
}

// DownstreamConfig — адреса соседних сервисов, которые зовёт оркестратор.
type DownstreamConfig struct {
	PyMLBaseURL  string        `env:"PYML_BASE_URL"      envDefault:"http://py-ml:8000"`
	APIBaseURL   string        `env:"API_BASE_URL"       envDefault:"http://go-api:8090"`
	AgentBaseURL string        `env:"AGENT_BASE_URL"     envDefault:"http://java-agent:8081"`
	Timeout      time.Duration `env:"DOWNSTREAM_TIMEOUT" envDefault:"30s"`
	// AgentTimeout длинный: агент делает PLAN → инструменты → DRAFT → CRITIC.
	AgentTimeout time.Duration `env:"AGENT_TIMEOUT" envDefault:"180s"`
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
