package newsfeed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html/charset"
)

// Часть лент (например Лента.ру) отдаёт пустой <description>: в RSS есть только
// заголовок и ссылка. Без текста новость бесполезна для анализа, поэтому
// дотягиваем страницу первоисточника и достаём из неё текст.
//
// Это сознательно простой экстрактор без зависимостей на readability-библиотеки:
// выкидываем скрипты, стили и разметку, собираем абзацы и оставляем длинные.

const (
	// minParagraphLen — короткие обрывки (меню, подписи, кнопки) в текст не берём.
	minParagraphLen = 80
	// maxArticleLen — ограничение на размер извлечённого текста.
	maxArticleLen = 20000
)

// Регулярки на каждый шумный тег отдельно: RE2 в Go не умеет обратные ссылки,
// поэтому одного выражения с \1 на закрывающий тег не получится.
var (
	noiseTagRes = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<script[^>]*>.*?</\s*script\s*>`),
		regexp.MustCompile(`(?is)<style[^>]*>.*?</\s*style\s*>`),
		regexp.MustCompile(`(?is)<noscript[^>]*>.*?</\s*noscript\s*>`),
		regexp.MustCompile(`(?is)<svg[^>]*>.*?</\s*svg\s*>`),
		regexp.MustCompile(`(?is)<iframe[^>]*>.*?</\s*iframe\s*>`),
	}
	paragraphRe = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</\s*p\s*>`)
)

// FetchArticle загружает страницу новости и возвращает её текст.
func (f *Fetcher) FetchArticle(ctx context.Context, articleURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, articleURL, nil)
	if err != nil {
		return "", fmt.Errorf("build article request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch article: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch article: unexpected status %d", resp.StatusCode)
	}

	// Перекодировка по заголовку и мета-тегу: часть сайтов до сих пор в cp1251.
	reader, err := charset.NewReader(io.LimitReader(resp.Body, maxBodySize), resp.Header.Get("Content-Type"))
	if err != nil {
		return "", fmt.Errorf("decode article charset: %w", err)
	}

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read article body: %w", err)
	}

	return extractArticleText(string(raw)), nil
}

// extractArticleText собирает текст статьи из абзацев страницы.
func extractArticleText(page string) string {
	for _, noiseRe := range noiseTagRes {
		page = noiseRe.ReplaceAllString(page, " ")
	}

	matches := paragraphRe.FindAllStringSubmatch(page, -1)
	paragraphs := make([]string, 0, len(matches))
	for _, match := range matches {
		text := cleanText(match[1])
		if len([]rune(text)) >= minParagraphLen {
			paragraphs = append(paragraphs, text)
		}
	}

	article := strings.Join(paragraphs, "\n")
	if len(article) > maxArticleLen {
		article = article[:maxArticleLen]
	}
	return article
}
