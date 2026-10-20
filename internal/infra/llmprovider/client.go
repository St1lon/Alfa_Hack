// Package llmprovider — HTTP-клиент к провайдеру языковых моделей хакатона.
// Ключ доступа живёт только здесь (внутри процесса llm-gateway) и не попадает
// ни в другие сервисы, ни в логи (CLAUDE.md §7, §9).
package llmprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
	completellm "github.com/st1lon/alfa-hack/internal/usecase/llm/complete"
)

// maxErrBody — сколько байт тела ошибки провайдера попадает в сообщение.
const maxErrBody = 512

// Config — параметры обращения к провайдеру.
type Config struct {
	// BaseURL — базовый адрес OpenAI-совместимого API (без /chat/completions).
	BaseURL string
	APIKey  string
	Timeout time.Duration
	// MaxRetries — количество повторов при сетевой ошибке или 5xx.
	MaxRetries int
	// RetryBackoff — пауза перед первым повтором, дальше удваивается.
	RetryBackoff time.Duration
}

// Client — клиент провайдера моделей.
type Client struct {
	cfg    Config
	client *http.Client
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// New создаёт клиента провайдера.
func New(cfg Config) *Client {
	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

// Complete выполняет chat-completion с ретраями на сетевые ошибки и 5xx.
func (c *Client) Complete(
	ctx context.Context, req *completellm.ProviderRequest,
) (*completellm.ProviderResponse, error) {
	payload, err := json.Marshal(toChatRequest(req))
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	backoff := c.cfg.RetryBackoff
	var lastErr error

	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("wait before retry: %w", ctx.Err())
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		response, retryable, callErr := c.do(ctx, payload)
		if callErr == nil {
			return response, nil
		}
		lastErr = callErr
		if !retryable {
			return nil, callErr
		}
	}

	return nil, lastErr
}

// do делает один запрос; второе значение — стоит ли повторять.
func (c *Client) do(ctx context.Context, payload []byte) (*completellm.ProviderResponse, bool, error) {
	endpoint := strings.TrimSuffix(c.cfg.BaseURL, "/") + "/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, false, fmt.Errorf("build chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, true, fmt.Errorf("call provider: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("read provider response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		retryable := httpResp.StatusCode >= http.StatusInternalServerError ||
			httpResp.StatusCode == http.StatusTooManyRequests
		return nil, retryable, fmt.Errorf("provider status %d: %s", httpResp.StatusCode, trimBody(body))
	}

	var parsed chatResponse
	if err = json.Unmarshal(body, &parsed); err != nil {
		return nil, false, fmt.Errorf("unmarshal provider response: %w", err)
	}
	if parsed.Error != nil {
		return nil, false, fmt.Errorf("provider error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, false, errors.New("provider returned no choices")
	}

	return &completellm.ProviderResponse{
		Content:      parsed.Choices[0].Message.Content,
		FinishReason: parsed.Choices[0].FinishReason,
		Usage: domain.LLMUsage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		},
	}, false, nil
}

func toChatRequest(req *completellm.ProviderRequest) chatRequest {
	messages := make([]chatMessage, 0, len(req.Messages))
	for _, message := range req.Messages {
		messages = append(messages, chatMessage{Role: message.Role, Content: message.Content})
	}
	return chatRequest{
		Model:          req.Model,
		Messages:       messages,
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
		ResponseFormat: req.ResponseFormat,
	}
}

func trimBody(body []byte) string {
	if len(body) > maxErrBody {
		return string(body[:maxErrBody])
	}
	return string(body)
}
