package newsfeed_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/services/go-ingest/internal/infra/newsfeed"
)

const articleSample = `<!DOCTYPE html>
<html><head><title>Новость</title>
<style>.ad { display: none }</style>
<script>window.counter = 1;</script>
</head>
<body>
  <nav><p>Главная</p><p>Экономика</p></nav>
  <article>
    <p>Совет директоров Банка России сохранил ключевую ставку на уровне 14 процентов годовых, отметив замедление инфляции.</p>
    <p>Регулятор указал, что решение связано с динамикой корпоративного кредитования и курсовыми ожиданиями экспортёров.</p>
  </article>
  <footer><p>Подписка</p></footer>
</body></html>`

func TestFetchArticleExtractsParagraphs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleSample))
	}))
	t.Cleanup(server.Close)

	fetcher := newsfeed.NewFetcher(5*time.Second, "test-agent")

	text, err := fetcher.FetchArticle(context.Background(), server.URL)
	require.NoError(t, err)

	// Содержательные абзацы попадают в текст.
	require.Contains(t, text, "сохранил ключевую ставку")
	require.Contains(t, text, "корпоративного кредитования")

	// Меню, подвал, скрипты и стили — нет.
	require.NotContains(t, text, "Главная")
	require.NotContains(t, text, "Подписка")
	require.NotContains(t, text, "window.counter")
	require.NotContains(t, text, "display: none")
	require.Len(t, strings.Split(text, "\n"), 2)
}

func TestFetchArticleFailsOnBadStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	fetcher := newsfeed.NewFetcher(5*time.Second, "test-agent")

	_, err := fetcher.FetchArticle(context.Background(), server.URL)
	require.Error(t, err)
	require.Contains(t, err.Error(), "404")
}
