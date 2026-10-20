package completellm

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// hashHeaderParts — сколько элементов ключа кэша идёт до сообщений:
// модель, задача, температура, лимит токенов, схема ответа.
const hashHeaderParts = 5

// Request — запрос к модели от сервиса команды.
type Request struct {
	// Service — кто вызывает: go-ingest | go-api | java-agent | py-ml.
	Service string
	// Actor — кто из команды гонял запрос (для разбивки дневного бюджета).
	Actor string
	// Task — задача маршрутизации; модель выбирается по ней, если Model пуст.
	Task domain.LLMTask
	// Model — явный выбор модели; должен быть в allowlist.
	Model       string
	Messages    []domain.Message
	Temperature float64
	MaxTokens   int
	// ResponseFormat пробрасывается провайдеру как есть (Structured Outputs у Java).
	ResponseFormat json.RawMessage
	UseCache       bool
	RequestID      string
}

func (req *Request) validate() error {
	req.Service = strings.TrimSpace(req.Service)
	req.Actor = strings.TrimSpace(req.Actor)
	req.Model = strings.TrimSpace(req.Model)

	if req.Service == "" {
		return domain.ErrValidation
	}
	if len(req.Messages) == 0 {
		return domain.ErrEmptyPrompt
	}
	for _, message := range req.Messages {
		if strings.TrimSpace(message.Content) != "" {
			return nil
		}
	}
	return domain.ErrEmptyPrompt
}

// promptHash — ключ кэша: модель, задача, параметры и содержимое запроса.
// Версия модели меняет ключ, поэтому старый кэш не «прилипает» к новой версии.
func (req *Request) promptHash(model string) string {
	parts := make([]string, 0, len(req.Messages)*2+hashHeaderParts)
	parts = append(parts,
		model,
		string(req.Task),
		strconv.FormatFloat(req.Temperature, 'f', 3, 64),
		strconv.Itoa(req.MaxTokens),
		string(req.ResponseFormat),
	)
	for _, message := range req.Messages {
		parts = append(parts, message.Role, message.Content)
	}
	return domain.PromptHash(parts...)
}
