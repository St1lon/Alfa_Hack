package analyzestories_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	analyzestories "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/analyze"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/stage"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/pipeline/stage/stagetest"
)

type stubRepo struct {
	stories   []domain.Story
	companies map[uuid.UUID][]string
	statuses  map[uuid.UUID]domain.PipelineStatus
	gotLimit  int
	gotThresh float64
}

func newRepo(companiesPerStory ...[]string) *stubRepo {
	repo := &stubRepo{companies: map[uuid.UUID][]string{}, statuses: map[uuid.UUID]domain.PipelineStatus{}}
	for _, companies := range companiesPerStory {
		story := domain.Story{ID: uuid.New(), Status: domain.StatusScored}
		repo.stories = append(repo.stories, story)
		repo.companies[story.ID] = companies
	}
	return repo
}

func (r *stubRepo) ClaimStories(context.Context, domain.PipelineStatus, string, int) ([]domain.Story, error) {
	return r.stories, nil
}

func (r *stubRepo) SetStoryStatus(_ context.Context, id uuid.UUID, status domain.PipelineStatus) error {
	r.statuses[id] = status
	return nil
}

func (r *stubRepo) RelevantCompanies(_ context.Context, id uuid.UUID, threshold float64, limit int) ([]string, error) {
	r.gotThresh, r.gotLimit = threshold, limit
	companies := r.companies[id]
	if len(companies) > limit {
		companies = companies[:limit]
	}
	return companies, nil
}

type stubAgent struct {
	calls    []string
	failFor  string
	notReady bool
}

func (a *stubAgent) CreateInsight(_ context.Context, _ uuid.UUID, companyID string) (string, error) {
	a.calls = append(a.calls, companyID)
	if a.notReady {
		return "", domain.ErrDownstreamNotReady
	}
	if companyID == a.failFor {
		return "", errors.New("agent 500")
	}
	return "insight-" + companyID, nil
}

var cfg = analyzestories.Config{RelevanceThreshold: 0.5, MaxCompanies: 2}

func TestAnalyzeFiltersStoryWithoutRelevantCompanies(t *testing.T) {
	t.Parallel()

	repo := newRepo(nil)
	agent := &stubAgent{}
	uc := analyzestories.NewUsecase(repo, agent, stagetest.NewAttempts(5), &stagetest.Tx{}, &stagetest.Events{}, cfg)

	result, err := uc.Execute(context.Background(), &stage.Request{Limit: 5})

	require.NoError(t, err)
	require.Equal(t, 1, result.Filtered)
	require.Equal(t, domain.StatusFiltered, repo.statuses[repo.stories[0].ID])
	require.Empty(t, agent.calls, "нерелевантный сюжет не стоит ни одного вызова модели")
}

func TestAnalyzeCapsCompaniesPerStory(t *testing.T) {
	t.Parallel()

	repo := newRepo([]string{"U_1", "U_2", "U_3", "U_4"})
	agent := &stubAgent{}
	uc := analyzestories.NewUsecase(repo, agent, stagetest.NewAttempts(5), &stagetest.Tx{}, &stagetest.Events{}, cfg)

	result, err := uc.Execute(context.Background(), &stage.Request{Limit: 5})

	require.NoError(t, err)
	require.Equal(t, 1, result.Processed)
	require.Equal(t, []string{"U_1", "U_2"}, agent.calls, "в агента — только топ-N компаний: бюджет")
	require.Equal(t, 2, repo.gotLimit)
	require.InDelta(t, 0.5, repo.gotThresh, 1e-9)
	require.Equal(t, domain.StatusAnalyzed, repo.statuses[repo.stories[0].ID])
}

func TestAnalyzeFailureKeepsStoryForRetry(t *testing.T) {
	t.Parallel()

	repo := newRepo([]string{"U_1", "U_2"})
	agent := &stubAgent{failFor: "U_2"}
	attempts := stagetest.NewAttempts(5)
	uc := analyzestories.NewUsecase(repo, agent, attempts, &stagetest.Tx{}, &stagetest.Events{}, cfg)

	result, err := uc.Execute(context.Background(), &stage.Request{Limit: 5})

	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Equal(t, 1, attempts.Total())
	_, touched := repo.statuses[repo.stories[0].ID]
	require.False(t, touched, "сюжет остаётся SCORED: на ретрае агент повторит обе пары идемпотентно")
}

func TestAnalyzeStopsWhenAgentNotReady(t *testing.T) {
	t.Parallel()

	repo := newRepo([]string{"U_1"}, []string{"U_2"})
	agent := &stubAgent{notReady: true}
	attempts := stagetest.NewAttempts(5)
	uc := analyzestories.NewUsecase(repo, agent, attempts, &stagetest.Tx{}, &stagetest.Events{}, cfg)

	result, err := uc.Execute(context.Background(), &stage.Request{Limit: 5})

	require.NoError(t, err)
	require.True(t, result.NotReady)
	require.Len(t, agent.calls, 1, "второй сюжет агенту не отправляем")
	require.Zero(t, attempts.Total())
	require.Empty(t, repo.statuses)
}
