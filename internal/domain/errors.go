package domain

import "errors"

// Sentinel-ошибки домена. Сравнение — только через errors.Is (CLAUDE.md §4).
var (
	// Новости и сюжеты.
	ErrNewsNotFound      = errors.New("news not found")
	ErrNewsAlreadyExists = errors.New("news already exists")
	ErrStoryNotFound     = errors.New("story not found")
	ErrEmptyNewsBody     = errors.New("news body must not be empty")
	ErrInvalidNewsInput  = errors.New("invalid news input")
	ErrInvalidNewsURL    = errors.New("invalid news url")

	// Аналитика банка.
	ErrResearchDocNotFound = errors.New("research document not found")
	ErrResearchChunkEmpty  = errors.New("research chunk must not be empty")
	ErrResearchFileMissing = errors.New("research file not found")
	ErrInvalidResearchSpec = errors.New("invalid research document spec")
	ErrResearchNoText      = errors.New("no usable text extracted from research document")

	// Компании.
	ErrCompanyNotFound = errors.New("company not found")

	// Инсайты.
	ErrInsightNotFound = errors.New("insight not found")

	// Пайплайн.
	ErrUnknownPipelineStatus   = errors.New("unknown pipeline status")
	ErrInvalidStatusTransition = errors.New("invalid pipeline status transition")

	// LLM-Gateway.
	ErrModelNotAllowed     = errors.New("model is not in allowlist")
	ErrEmptyPrompt         = errors.New("prompt must not be empty")
	ErrUnknownLLMTask      = errors.New("unknown llm task")
	ErrBudgetExceeded      = errors.New("daily llm budget exceeded")
	ErrUpstreamUnavailable = errors.New("llm upstream unavailable")

	// Общее.
	ErrValidation = errors.New("validation error")
)
