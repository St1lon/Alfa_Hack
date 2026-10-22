package downstream

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// ───────────────────────── py-ml: POST /v1/enrich ─────────────────────────

// PyML — клиент к py-ml-gatekeeper (Python).
type PyML struct{ c client }

// NewPyML создаёт клиента к py-ml.
func NewPyML(baseURL string, timeout time.Duration) *PyML {
	return &PyML{c: newClient(baseURL, timeout)}
}

type enrichRequest struct {
	NewsID string `json:"news_id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type enrichResponse struct {
	Embedding []float32 `json:"embedding"`
	Entities  []struct {
		Type     string `json:"type"`
		Value    string `json:"value"`
		Mentions int    `json:"mentions"`
	} `json:"entities"`
	OKVEDHints []struct {
		Code   string  `json:"code"`
		Label  string  `json:"label"`
		Weight float64 `json:"weight"`
	} `json:"okved_hints"`
	NoiseScore   float64 `json:"noise_score"`
	ModelVersion string  `json:"model_version"`
}

// Enrich отправляет новость в py-ml и возвращает обогащение. В базу пишет
// оркестратор, а не py-ml: у перехода DEDUPED → ENRICHED один владелец.
func (p *PyML) Enrich(ctx context.Context, news *domain.News) (*domain.Enrichment, error) {
	var resp enrichResponse
	err := p.c.postJSON(ctx, "/v1/enrich", enrichRequest{
		NewsID: news.ID.String(), Title: news.Title, Body: news.Body,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if resp.ModelVersion == "" {
		return nil, errors.New("py-ml enrich: model_version is required")
	}

	enrichment := &domain.Enrichment{
		NewsID:       news.ID,
		Embedding:    resp.Embedding,
		NoiseScore:   resp.NoiseScore,
		ModelVersion: resp.ModelVersion,
	}
	for _, e := range resp.Entities {
		enrichment.Entities = append(enrichment.Entities, domain.Entity{
			Type: domain.EntityType(e.Type), Value: e.Value, Mentions: e.Mentions,
		})
	}
	for _, h := range resp.OKVEDHints {
		enrichment.OKVEDHints = append(enrichment.OKVEDHints, domain.OKVEDHint{
			Code: h.Code, Label: h.Label, Weight: h.Weight,
		})
	}
	return enrichment, nil
}

// ───────────────────── go-api: POST /internal/v1/stories/{id}/score ─────────────────────

// API — клиент к go-api (Go #2): подбор компаний к сюжету.
type API struct{ c client }

// NewAPI создаёт клиента к go-api.
func NewAPI(baseURL string, timeout time.Duration) *API {
	return &API{c: newClient(baseURL, timeout)}
}

type scoreResponse struct {
	Companies int `json:"companies"`
}

// ScoreStory просит go-api подобрать компании к сюжету. go-api сам пишет
// story_company_scores (это его таблица) и отвечает, сколько компаний нашлось.
func (a *API) ScoreStory(ctx context.Context, storyID uuid.UUID) (int, error) {
	var resp scoreResponse
	if err := a.c.postJSON(ctx, "/internal/v1/stories/"+storyID.String()+"/score", struct{}{}, &resp); err != nil {
		return 0, err
	}
	return resp.Companies, nil
}

// ───────────────────────── java-agent: POST /v1/insights ─────────────────────────

// Agent — клиент к java-agent: разбор пары «сюжет — компания».
type Agent struct{ c client }

// NewAgent создаёт клиента к java-agent. Таймаут длинный: агент делает
// PLAN → инструменты → DRAFT → CRITIC, это десятки секунд.
func NewAgent(baseURL string, timeout time.Duration) *Agent {
	return &Agent{c: newClient(baseURL, timeout)}
}

type insightRequest struct {
	StoryID   string `json:"story_id"`
	CompanyID string `json:"company_id"`
}

type insightResponse struct {
	InsightID string `json:"insight_id"`
}

// CreateInsight просит агента построить разбор. Java пишет insights, evidence
// и agent_runs сама; повторный вызов для той же пары должен быть идемпотентным
// (уникальный ключ story_id, company_id, model_version в insights).
func (a *Agent) CreateInsight(ctx context.Context, storyID uuid.UUID, companyID string) (string, error) {
	var resp insightResponse
	if err := a.c.postJSON(ctx, "/v1/insights", insightRequest{
		StoryID: storyID.String(), CompanyID: companyID,
	}, &resp); err != nil {
		return "", err
	}
	return resp.InsightID, nil
}
