package ingestnews

import (
	"strings"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// minBodyLen — ниже этого порога текст не несёт содержания для анализа
// (анонс из двух слов, заглушка пейволла).
const minBodyLen = 40

// Request — новость на входе: из ленты (SourceID задан) или вброшенная руками.
type Request struct {
	SourceID    *int16
	URL         string
	Title       string
	Body        string
	Publisher   string
	Lang        string
	PublishedAt *time.Time
}

func (req *Request) validate() error {
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	req.URL = strings.TrimSpace(req.URL)
	req.Publisher = strings.TrimSpace(req.Publisher)

	if req.Title == "" {
		return domain.ErrInvalidNewsInput
	}
	if len([]rune(req.Body)) < minBodyLen {
		return domain.ErrEmptyNewsBody
	}
	return nil
}
