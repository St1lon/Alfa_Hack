package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"

	sharedconfig "github.com/st1lon/alfa-hack/internal/config"
)

// ResearchLoader — конфигурация утилиты research-loader (задача G1-13).
type ResearchLoader struct {
	PG       sharedconfig.PostgresConfig `envPrefix:"ALFA_"`
	Research ResearchConfig              `envPrefix:"RESEARCH_"`
	LogLevel string                      `env:"RESEARCH_LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
}

// ResearchConfig — откуда брать материалы, как их распознавать и резать.
type ResearchConfig struct {
	Dir          string `env:"DIR"      envDefault:"data/research" validate:"required"`
	ManifestFile string `env:"MANIFEST" envDefault:"configs/research.yaml" validate:"required"`
	// Force — перезалить документы, даже если их чанки уже в базе.
	Force bool `env:"FORCE" envDefault:"false"`

	OCRLang        string        `env:"OCR_LANG"         envDefault:"rus+eng" validate:"required"`
	OCRDPI         int           `env:"OCR_DPI"          envDefault:"200" validate:"min=72,max=600"`
	OCRWorkers     int           `env:"OCR_WORKERS"      envDefault:"4" validate:"min=1,max=32"`
	OCRPageTimeout time.Duration `env:"OCR_PAGE_TIMEOUT" envDefault:"120s"`
	// OCRCacheDir — кэш распознанных страниц; пусто — без кэша.
	OCRCacheDir string `env:"OCR_CACHE_DIR" envDefault:"data/research/.ocr-cache"`
	// EnrichmentDictionary — словарь классификации чанков, тот же, что у новостей.
	EnrichmentDictionary string `env:"ENRICHMENT_DICTIONARY" envDefault:"configs/enrichment.yaml"`

	// Размеры чанков в символах: ~3 символа на токен, итого 300–600 токенов.
	ChunkMinRunes    int `env:"CHUNK_MIN_RUNES"   envDefault:"900" validate:"min=1"`
	ChunkTargetRunes int `env:"CHUNK_TARGET_RUNES" envDefault:"1350" validate:"min=1"`
	ChunkMaxRunes    int `env:"CHUNK_MAX_RUNES"   envDefault:"1800" validate:"min=1"`
	ChunkMinLetters  int `env:"CHUNK_MIN_LETTERS" envDefault:"120" validate:"min=0"`
}

// ResearchManifest — содержимое configs/research.yaml.
type ResearchManifest struct {
	Documents []ResearchManifestEntry `yaml:"documents"`
}

// ResearchManifestEntry — один материал ЦМИ.
type ResearchManifestEntry struct {
	Code  string `yaml:"code"`
	Title string `yaml:"title"`
	File  string `yaml:"file"`
	// PublishedAt — дата в формате 2006-01-02; пусто, если в материале её нет.
	PublishedAt string `yaml:"published_at"`
}

// LoadResearchLoader читает конфигурацию research-loader.
func LoadResearchLoader(validate *validator.Validate) (*ResearchLoader, error) {
	var cfg ResearchLoader
	if err := sharedconfig.Parse(&cfg, validate); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	r := cfg.Research
	if r.ChunkMinRunes > r.ChunkTargetRunes || r.ChunkTargetRunes > r.ChunkMaxRunes {
		return nil, errors.New("load config: need RESEARCH_CHUNK_MIN_RUNES <= TARGET <= MAX")
	}
	return &cfg, nil
}

// LoadResearchManifest читает список материалов.
func LoadResearchManifest(path string) (*ResearchManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read research manifest: %w", err)
	}

	var manifest ResearchManifest
	if err = yaml.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal research manifest: %w", err)
	}
	if len(manifest.Documents) == 0 {
		return nil, fmt.Errorf("research manifest %s: no documents", path)
	}
	return &manifest, nil
}

// PublishedDate разбирает дату публикации. ok=false — даты в материале нет,
// это штатный случай (например, у обзора золота указан только месяц).
func (e ResearchManifestEntry) PublishedDate() (date time.Time, ok bool, err error) {
	if e.PublishedAt == "" {
		return time.Time{}, false, nil
	}
	date, err = time.Parse(time.DateOnly, e.PublishedAt)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("document %s: bad published_at %q: %w", e.Code, e.PublishedAt, err)
	}
	return date, true, nil
}
