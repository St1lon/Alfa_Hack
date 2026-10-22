package ingestapp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/config"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/infra/enricher"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/infra/ocr"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/repository"
	loadresearch "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/research/load"
)

// RunResearchLoader — сборка и однократный запуск загрузчика аналитики ЦМИ.
// Вызывается из cmd/research-loader; в compose это one-shot job после миграций.
func RunResearchLoader(ctx context.Context, cfg *config.ResearchLoader, log *slog.Logger) (*loadresearch.Result, error) {
	req, err := researchRequest(cfg)
	if err != nil {
		return nil, err
	}

	// Тот же словарь, что у новостей: агент сопоставит тему новости и чанка напрямую.
	classifier, err := enricher.LoadDictionary(cfg.Research.EnrichmentDictionary)
	if err != nil {
		return nil, fmt.Errorf("load enrichment dictionary: %w", err)
	}

	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{
		Host:     cfg.PG.Host,
		Port:     cfg.PG.Port,
		User:     cfg.PG.User,
		Password: cfg.PG.Password,
		Database: cfg.PG.Database,
		SSLMode:  cfg.PG.SSLMode,
		MaxConns: cfg.PG.MaxConns,
	})
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}
	defer pool.Close()

	extractor := ocr.New(ocr.Config{
		DPI:         cfg.Research.OCRDPI,
		Lang:        cfg.Research.OCRLang,
		Workers:     cfg.Research.OCRWorkers,
		PageTimeout: cfg.Research.OCRPageTimeout,
		CacheDir:    cfg.Research.OCRCacheDir,
	}, log)

	usecase := loadresearch.NewUsecase(
		repository.NewResearchRepo(pool),
		extractor,
		classifier,
		postgres.NewTxManager(pool),
		repository.NewPipelineRepo(pool),
		loadresearch.ChunkConfig{
			MinRunes:    cfg.Research.ChunkMinRunes,
			TargetRunes: cfg.Research.ChunkTargetRunes,
			MaxRunes:    cfg.Research.ChunkMaxRunes,
			MinLetters:  cfg.Research.ChunkMinLetters,
		},
	)

	result, err := usecase.Execute(ctx, req)
	if err != nil {
		return result, fmt.Errorf("load research: %w", err)
	}
	return result, nil
}

// researchRequest собирает запрос usecase из манифеста.
func researchRequest(cfg *config.ResearchLoader) (*loadresearch.Request, error) {
	manifest, err := config.LoadResearchManifest(cfg.Research.ManifestFile)
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}

	specs := make([]loadresearch.DocumentSpec, 0, len(manifest.Documents))
	for _, entry := range manifest.Documents {
		spec := loadresearch.DocumentSpec{Code: entry.Code, Title: entry.Title, File: entry.File}
		date, ok, dateErr := entry.PublishedDate()
		if dateErr != nil {
			return nil, fmt.Errorf("load manifest: %w", dateErr)
		}
		if ok {
			spec.PublishedAt = &date
		}
		specs = append(specs, spec)
	}

	return &loadresearch.Request{
		Dir:       cfg.Research.Dir,
		Documents: specs,
		Force:     cfg.Research.Force,
	}, nil
}
