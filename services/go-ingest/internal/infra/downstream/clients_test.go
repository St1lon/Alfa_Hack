package downstream_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/infra/downstream"
)

func serve(t *testing.T, status int, body string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestNotImplementedMeansNotReady(t *testing.T) {
	t.Parallel()

	// Ровно так сейчас отвечают заглушки py-ml, go-api и java-agent.
	for _, status := range []int{http.StatusNotImplemented, http.StatusNotFound} {
		url := serve(t, status, `{"code":"NOT_IMPLEMENTED"}`)

		_, err := downstream.NewPyML(url, time.Second).Enrich(context.Background(), &domain.News{ID: uuid.New()})
		require.ErrorIs(t, err, domain.ErrDownstreamNotReady)

		_, err = downstream.NewAPI(url, time.Second).ScoreStory(context.Background(), uuid.New())
		require.ErrorIs(t, err, domain.ErrDownstreamNotReady)

		_, err = downstream.NewAgent(url, time.Second).CreateInsight(context.Background(), uuid.New(), "U_1")
		require.ErrorIs(t, err, domain.ErrDownstreamNotReady)
	}
}

func TestUnreachableMeansNotReady(t *testing.T) {
	t.Parallel()

	// Порт, на котором никто не слушает: сервис не поднят.
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	_, err := downstream.NewAPI(url, time.Second).ScoreStory(context.Background(), uuid.New())
	require.ErrorIs(t, err, domain.ErrDownstreamNotReady)

	_, err = downstream.NewAPI("", time.Second).ScoreStory(context.Background(), uuid.New())
	require.ErrorIs(t, err, domain.ErrDownstreamNotReady, "адрес не настроен — тоже не готов")
}

func TestServerErrorIsRetriable(t *testing.T) {
	t.Parallel()

	url := serve(t, http.StatusInternalServerError, `{"error":"boom"}`)

	_, err := downstream.NewAgent(url, time.Second).CreateInsight(context.Background(), uuid.New(), "U_1")
	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrDownstreamNotReady, "500 — сбой конкретной записи, её ретраим")
	require.Contains(t, err.Error(), "boom")
}

func TestPyMLEnrichParsesContract(t *testing.T) {
	t.Parallel()

	newsID := uuid.New()
	// Хендлер работает не в горутине теста, поэтому require внутри него нельзя:
	// запоминаем запрос и проверяем его после вызова.
	var (
		gotPath string
		gotBody map[string]string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		_, _ = w.Write([]byte(`{
			"embedding": [0.1, 0.2],
			"entities": [{"type": "COUNTRY", "value": "КИТАЙ", "mentions": 2}],
			"okved_hints": [{"code": "24", "label": "Металлургия", "weight": 1}],
			"noise_score": 0.1,
			"model_version": "pyml-v1"
		}`))
	}))
	t.Cleanup(server.Close)

	result, err := downstream.NewPyML(server.URL, time.Second).Enrich(context.Background(),
		&domain.News{ID: newsID, Title: "Экспорт стали в Китай", Body: "текст"})

	require.NoError(t, err)
	require.Equal(t, "/v1/enrich", gotPath)
	require.Equal(t, newsID.String(), gotBody["news_id"])
	require.Equal(t, "Экспорт стали в Китай", gotBody["title"])
	require.Equal(t, newsID, result.NewsID)
	require.Equal(t, []float32{0.1, 0.2}, result.Embedding)
	require.Equal(t, "КИТАЙ", result.Entities[0].Value)
	require.Equal(t, domain.EntityCountry, result.Entities[0].Type)
	require.Equal(t, "24", result.OKVEDHints[0].Code)
	require.Equal(t, "pyml-v1", result.ModelVersion)
}

func TestAgentAndAPIParseResponses(t *testing.T) {
	t.Parallel()

	agentURL := serve(t, http.StatusCreated, `{"insight_id":"ins-1"}`)
	insightID, err := downstream.NewAgent(agentURL, time.Second).CreateInsight(context.Background(), uuid.New(), "U_1")
	require.NoError(t, err)
	require.Equal(t, "ins-1", insightID)

	apiURL := serve(t, http.StatusOK, `{"companies":3}`)
	companies, err := downstream.NewAPI(apiURL, time.Second).ScoreStory(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, 3, companies)
}
