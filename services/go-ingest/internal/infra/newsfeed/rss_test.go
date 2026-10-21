package newsfeed_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/services/go-ingest/internal/infra/newsfeed"
)

const rssSample = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Интерфакс</title>
    <item>
      <title>ЦБ сохранил ключевую ставку на уровне 14%</title>
      <link>https://www.interfax.ru/business/1</link>
      <description>&lt;p&gt;Совет директоров &lt;b&gt;сохранил&lt;/b&gt; ставку.&lt;/p&gt;</description>
      <pubDate>Mon, 19 Oct 2026 08:11:00 +0300</pubDate>
    </item>
    <item>
      <title>РЖД индексирует тарифы</title>
      <link>https://www.interfax.ru/business/2</link>
      <description>Тариф вырастет на 10% с 1 декабря.</description>
      <pubDate>Mon, 19 Oct 2026 09:00:00 +0300</pubDate>
    </item>
  </channel>
</rss>`

const atomSample = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>ТАСС</title>
  <entry>
    <title>Экспортные пошлины пересмотрят</title>
    <link rel="alternate" href="https://tass.ru/ekonomika/3"/>
    <summary>Минфин готовит изменение формулы пошлины.</summary>
    <published>2026-10-19T06:00:00Z</published>
  </entry>
</feed>`

func serve(t *testing.T, body, contentType string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestFetchParsesRSS(t *testing.T) {
	t.Parallel()

	server := serve(t, rssSample, "application/rss+xml")
	fetcher := newsfeed.NewFetcher(5*time.Second, "test-agent")

	items, err := fetcher.Fetch(context.Background(), server.URL)
	require.NoError(t, err)
	require.Len(t, items, 2)

	first := items[0]
	require.Equal(t, "ЦБ сохранил ключевую ставку на уровне 14%", first.Title)
	require.Equal(t, "https://www.interfax.ru/business/1", first.Link)
	require.Equal(t, "Интерфакс", first.Publisher)
	// HTML-разметка и сущности снимаются, пробелы схлопываются.
	require.Equal(t, "Совет директоров сохранил ставку.", first.Body)
	require.NotNil(t, first.PublishedAt)
	require.Equal(t, 2026, first.PublishedAt.Year())
	require.Equal(t, time.UTC, first.PublishedAt.Location())
}

func TestFetchParsesAtom(t *testing.T) {
	t.Parallel()

	server := serve(t, atomSample, "application/atom+xml")
	fetcher := newsfeed.NewFetcher(5*time.Second, "test-agent")

	items, err := fetcher.Fetch(context.Background(), server.URL)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// В Atom ссылка лежит в атрибуте href, а не в тексте элемента.
	require.Equal(t, "https://tass.ru/ekonomika/3", items[0].Link)
	require.Equal(t, "ТАСС", items[0].Publisher)
	require.NotNil(t, items[0].PublishedAt)
}

func TestFetchFailsOnBadStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	fetcher := newsfeed.NewFetcher(5*time.Second, "test-agent")

	_, err := fetcher.Fetch(context.Background(), server.URL)
	require.Error(t, err)
	require.Contains(t, err.Error(), "403")
}
