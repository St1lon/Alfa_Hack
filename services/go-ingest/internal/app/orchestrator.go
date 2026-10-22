package ingestapp

import (
	"fmt"
	"log/slog"

	"github.com/st1lon/alfa-hack/internal/infra/db/postgres"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/config"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/infra/downstream"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/infra/enricher"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/repository"
	analyzestories "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/analyze"
	clusternews "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/cluster"
	dedupenews "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/dedupe"
	enrichnews "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/enrich"
	scorestories "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/score"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/worker/orchestrator"
)

// buildOrchestrator собирает воркер пайплайна (G1-14) со всеми этапами.
func buildOrchestrator(
	cfg *config.Config, pool postgres.Pool, events *repository.PipelineRepo, log *slog.Logger,
) (*orchestrator.Worker, error) {
	orch := cfg.Orchestrator
	txm := postgres.NewTxManager(pool)
	flow := repository.NewFlowRepo(pool)
	stories := repository.NewStoryRepo(pool, repository.ClusterPolicy{
		Similarity: orch.ClusterSimilarity,
		Window:     orch.ClusterWindow,
	})
	attempts := repository.NewAttemptsRepo(pool, repository.RetryPolicy{
		MaxAttempts: orch.MaxAttempts,
		BackoffBase: orch.BackoffBase,
		BackoffMax:  orch.BackoffMax,
	})

	newsEnricher, err := buildEnricher(cfg, log)
	if err != nil {
		return nil, err
	}

	dedupe := dedupenews.NewUsecase(flow, txm, events)
	enrich := enrichnews.NewUsecase(flow, newsEnricher, attempts, txm, events,
		enrichnews.Config{NoiseThreshold: orch.NoiseThreshold})
	cluster := clusternews.NewUsecase(flow, stories, txm, events)
	score := scorestories.NewUsecase(stories,
		downstream.NewAPI(cfg.Downstream.APIBaseURL, cfg.Downstream.Timeout), attempts, txm, events)
	analyze := analyzestories.NewUsecase(stories,
		downstream.NewAgent(cfg.Downstream.AgentBaseURL, cfg.Downstream.AgentTimeout), attempts, txm, events,
		analyzestories.Config{RelevanceThreshold: orch.RelevanceThreshold, MaxCompanies: orch.MaxCompaniesPerStory})

	stages := []orchestrator.Stage{
		{Name: dedupenews.Name, Batch: orch.Batch, Run: dedupe.Execute},
		{Name: enrichnews.Name, Batch: orch.Batch, Run: enrich.Execute},
		{Name: clusternews.Name, Batch: orch.Batch, Run: cluster.Execute},
		{Name: scorestories.Name, Batch: orch.Batch, Run: score.Execute},
		{Name: analyzestories.Name, Batch: orch.AnalyzeBatch, Run: analyze.Execute},
	}

	return orchestrator.New(orchestrator.Config{
		Tick:              orch.Tick,
		MaxBatchesPerTick: orch.MaxBatchesPerTick,
	}, stages, log), nil
}

// buildEnricher выбирает обогатитель: словарный fallback или py-ml.
func buildEnricher(cfg *config.Config, log *slog.Logger) (enrichnews.Enricher, error) {
	if !cfg.UseMLFallback {
		log.Info("news enrichment via py-ml", slog.String("url", cfg.Downstream.PyMLBaseURL))
		return downstream.NewPyML(cfg.Downstream.PyMLBaseURL, cfg.Downstream.Timeout), nil
	}

	lexical, err := enricher.LoadDictionary(cfg.Orchestrator.EnrichmentDictionary)
	if err != nil {
		return nil, fmt.Errorf("load enrichment dictionary: %w", err)
	}
	log.Info("news enrichment via go fallback dictionaries (USE_ML_FALLBACK=true)",
		slog.String("dictionary", cfg.Orchestrator.EnrichmentDictionary))
	return lexical, nil
}
