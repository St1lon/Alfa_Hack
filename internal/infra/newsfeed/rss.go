// Package newsfeed читает внешние ленты новостей (RSS 2.0 и Atom) и приводит
// элементы к одному виду. Сетевые ошибки оборачиваются, таймаут — из конфига.
package newsfeed

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

// maxBodySize — защита от гигантского ответа ленты (10 МиБ).
const maxBodySize = 10 << 20

// tagRe вырезает HTML-разметку из описания элемента ленты.
var tagRe = regexp.MustCompile(`(?s)<[^>]*>`)

// dateLayouts — форматы даты, встречающиеся в российских лентах.
var dateLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC3339,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 02 Jan 2006 15:04:05 MST",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02 15:04:05",
}

// Item — элемент ленты, приведённый к общему виду.
type Item struct {
	Title       string
	Link        string
	Body        string
	Publisher   string
	PublishedAt *time.Time
}

// feed — разбор RSS 2.0 и Atom одной структурой: теги не конфликтуют.
type feed struct {
	Channel struct {
		Title string     `xml:"title"`
		Items []feedItem `xml:"item"`
	} `xml:"channel"`
	Title   string     `xml:"title"`
	Entries []feedItem `xml:"entry"`
}

type feedItem struct {
	Title string `xml:"title"`
	// Links покрывает оба формата: в RSS ссылка лежит текстом внутри <link>,
	// в Atom — в атрибуте href, и элементов <link> там может быть несколько.
	Links       []feedLink `xml:"link"`
	Description string     `xml:"description"`
	Summary     string     `xml:"summary"`
	Content     string     `xml:"content"`
	Encoded     string     `xml:"encoded"` // content:encoded
	FullText    string     `xml:"full-text"`
	PubDate     string     `xml:"pubDate"`
	Published   string     `xml:"published"`
	Updated     string     `xml:"updated"`
	DCDate      string     `xml:"date"` // dc:date
}

type feedLink struct {
	Href  string `xml:"href,attr"`
	Value string `xml:",chardata"`
}

// Fetcher тянет и разбирает ленты.
type Fetcher struct {
	client    *http.Client
	userAgent string
}

// NewFetcher создаёт загрузчик лент с таймаутом на запрос.
func NewFetcher(timeout time.Duration, userAgent string) *Fetcher {
	return &Fetcher{
		client:    &http.Client{Timeout: timeout},
		userAgent: userAgent,
	}
}

// Fetch загружает ленту по URL и возвращает её элементы.
func (f *Fetcher) Fetch(ctx context.Context, feedURL string) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build feed request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch feed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch feed: unexpected status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("read feed body: %w", err)
	}

	return parse(raw)
}

// parse разбирает тело ленты в элементы.
func parse(raw []byte) ([]Item, error) {
	var parsed feed

	decoder := xml.NewDecoder(bytes.NewReader(raw))
	// Часть российских лент (например Финмаркет) отдаёт windows-1251.
	// Без перекодировки декодер падает на "invalid UTF-8" и источник теряется.
	decoder.CharsetReader = charset.NewReaderLabel
	decoder.Strict = false

	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode feed xml: %w", err)
	}

	publisher := strings.TrimSpace(parsed.Channel.Title)
	if publisher == "" {
		publisher = strings.TrimSpace(parsed.Title)
	}

	rawItems := parsed.Channel.Items
	if len(rawItems) == 0 {
		rawItems = parsed.Entries
	}

	items := make([]Item, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item := Item{
			Title:       cleanText(rawItem.Title),
			Link:        itemLink(rawItem),
			Body:        cleanText(itemBody(rawItem)),
			Publisher:   publisher,
			PublishedAt: itemDate(rawItem),
		}
		if item.Title == "" {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func itemLink(item feedItem) string {
	for _, link := range item.Links {
		if value := strings.TrimSpace(link.Value); value != "" {
			return value
		}
	}
	for _, link := range item.Links {
		if href := strings.TrimSpace(link.Href); href != "" {
			return href
		}
	}
	return ""
}

// itemBody берёт самый полный доступный текст элемента.
func itemBody(item feedItem) string {
	candidates := []string{item.FullText, item.Encoded, item.Content, item.Description, item.Summary}
	best := ""
	for _, candidate := range candidates {
		if len(candidate) > len(best) {
			best = candidate
		}
	}
	return best
}

func itemDate(item feedItem) *time.Time {
	for _, value := range []string{item.PubDate, item.Published, item.Updated, item.DCDate} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		for _, layout := range dateLayouts {
			if parsed, err := time.Parse(layout, value); err == nil {
				utc := parsed.UTC()
				return &utc
			}
		}
	}
	return nil
}

// cleanText снимает разметку и схлопывает пробелы.
func cleanText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}
