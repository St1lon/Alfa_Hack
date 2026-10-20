// Package apierrors транслирует domain sentinel-ошибки в HTTP-статусы
// и структурированные коды ответа (CLAUDE.md §4).
package apierrors

import (
	"errors"
	"net/http"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// Коды ошибок API. SCREAMING_SNAKE, стабильны — на них опирается фронт.
const (
	CodeBadRequest      = "BAD_REQUEST"
	CodeInvalidNews     = "INVALID_NEWS"
	CodeEmptyNewsBody   = "EMPTY_NEWS_BODY"
	CodeInvalidNewsURL  = "INVALID_NEWS_URL"
	CodeNotFound        = "NOT_FOUND"
	CodeEmptyPrompt     = "EMPTY_PROMPT"
	CodeModelNotAllowed = "MODEL_NOT_ALLOWED"
	CodeUnknownTask     = "UNKNOWN_TASK"
	CodeBudgetExceeded  = "BUDGET_EXCEEDED"
	CodeUpstreamFailed  = "UPSTREAM_FAILED"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeInternal        = "INTERNAL"
)

// Body — тело ответа об ошибке: {code, description}.
type Body struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// Map возвращает HTTP-статус и тело для ошибки usecase.
func Map(err error) (int, Body) {
	switch {
	case errors.Is(err, domain.ErrInvalidNewsInput), errors.Is(err, domain.ErrValidation):
		return http.StatusBadRequest, Body{CodeInvalidNews, err.Error()}
	case errors.Is(err, domain.ErrEmptyNewsBody):
		return http.StatusBadRequest, Body{CodeEmptyNewsBody, err.Error()}
	case errors.Is(err, domain.ErrInvalidNewsURL):
		return http.StatusBadRequest, Body{CodeInvalidNewsURL, err.Error()}
	case errors.Is(err, domain.ErrEmptyPrompt):
		return http.StatusBadRequest, Body{CodeEmptyPrompt, err.Error()}
	case errors.Is(err, domain.ErrModelNotAllowed):
		return http.StatusBadRequest, Body{CodeModelNotAllowed, err.Error()}
	case errors.Is(err, domain.ErrUnknownLLMTask):
		return http.StatusBadRequest, Body{CodeUnknownTask, err.Error()}
	case errors.Is(err, domain.ErrNewsNotFound),
		errors.Is(err, domain.ErrStoryNotFound),
		errors.Is(err, domain.ErrCompanyNotFound),
		errors.Is(err, domain.ErrInsightNotFound):
		return http.StatusNotFound, Body{CodeNotFound, err.Error()}
	case errors.Is(err, domain.ErrBudgetExceeded):
		// 429: дневной лимит исчерпан, повтор имеет смысл только после сброса бюджета.
		return http.StatusTooManyRequests, Body{CodeBudgetExceeded, err.Error()}
	case errors.Is(err, domain.ErrUpstreamUnavailable):
		return http.StatusBadGateway, Body{CodeUpstreamFailed, err.Error()}
	default:
		// Наружу не отдаём текст неизвестной ошибки — он уже в логе сервиса.
		return http.StatusInternalServerError, Body{CodeInternal, "internal error"}
	}
}

// BadRequest — ответ на нечитаемый запрос (битый JSON, параметры).
func BadRequest(description string) Body {
	return Body{Code: CodeBadRequest, Description: description}
}

// Unauthorized — ответ на запрос без валидного внутреннего токена.
func Unauthorized() Body {
	return Body{Code: CodeUnauthorized, Description: "invalid or missing service token"}
}
