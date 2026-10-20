package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"unicode"
)

// trackingPrefixes — параметры-метки, которые не меняют содержимое страницы.
// Без их отбрасывания одна и та же новость приходит как несколько «разных» URL.
var trackingPrefixes = []string{"utm_", "yclid", "gclid", "fbclid", "from", "_openstat"}

// CanonicalURL приводит ссылку к канонному виду: нижний регистр схемы и хоста,
// без www, без фрагмента, без трекинговых параметров, с отсортированным query.
// Пустая строка на входе даёт пустую строку на выходе (вброс новости текстом).
func CanonicalURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalidNewsURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidNewsURL
	}
	if parsed.Host == "" {
		return "", ErrInvalidNewsURL
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.TrimPrefix(strings.ToLower(parsed.Host), "www.")
	parsed.Fragment = ""
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")

	query := parsed.Query()
	for key := range query {
		if isTrackingParam(key) {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode() // Encode сортирует ключи

	return parsed.String(), nil
}

// NormalizeText приводит текст к виду, устойчивому к перепечаткам с косметическими
// правками: нижний регистр, схлопнутые пробелы, без пунктуации.
func NormalizeText(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))

	lastSpace := true
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastSpace = false
		case !lastSpace:
			builder.WriteRune(' ')
			lastSpace = true
		}
	}

	return strings.TrimSpace(builder.String())
}

// TextHash — sha256 нормализованного «заголовок + тело». Основной ключ дедупликации.
func TextHash(title, body string) string {
	return sha256Hex(NormalizeText(title) + " " + NormalizeText(body))
}

// URLHash — sha256 канонизированного URL. Пустой URL даёт пустой хэш:
// уникальный индекс в БД построен с условием url_hash IS NOT NULL.
func URLHash(canonicalURL string) string {
	if canonicalURL == "" {
		return ""
	}
	return sha256Hex(canonicalURL)
}

// PromptHash — ключ кэша LLM-Gateway: модель, задача и содержимое запроса.
func PromptHash(parts ...string) string {
	return sha256Hex(strings.Join(parts, "\x00"))
}

func isTrackingParam(key string) bool {
	lower := strings.ToLower(key)
	for _, prefix := range trackingPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
