// Package downstream — HTTP-клиенты оркестратора к соседним сервисам:
// py-ml (обогащение), go-api (подбор компаний), java-agent (разбор).
//
// Общее правило для всех трёх: если сервис ещё не реализовал эндпоинт
// (501/404) или недоступен вовсе, клиент возвращает domain.ErrDownstreamNotReady —
// оркестратор пропустит этап до следующего тика, не сжигая попытки.
// Остальные ошибки — это сбой конкретной записи, она уходит в ретраи.
package downstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// maxErrBody — сколько байт тела ответа об ошибке попадает в сообщение.
const maxErrBody = 300

// client — общий транспорт трёх клиентов.
type client struct {
	baseURL string
	http    *http.Client
}

func newClient(baseURL string, timeout time.Duration) client {
	return client{baseURL: strings.TrimSuffix(baseURL, "/"), http: &http.Client{Timeout: timeout}}
}

// postJSON отправляет body и разбирает ответ 2xx в out (если out не nil).
func (c client) postJSON(ctx context.Context, path string, body, out any) error {
	if c.baseURL == "" {
		return fmt.Errorf("%w: base url is not configured", domain.ErrDownstreamNotReady)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		if isUnreachable(err) {
			return fmt.Errorf("%w: %w", domain.ErrDownstreamNotReady, err)
		}
		return fmt.Errorf("call %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusNotImplemented || resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s answered %d", domain.ErrDownstreamNotReady, path, resp.StatusCode)
	case resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices:
		return fmt.Errorf("call %s: status %d: %s", path, resp.StatusCode, trim(raw))
	}

	if out == nil {
		return nil
	}
	if err = json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("unmarshal %s response: %w", path, err)
	}
	return nil
}

// isUnreachable — сервис не поднят или не резолвится: ждём его, а не ретраим запись.
func isUnreachable(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return true
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

func trim(body []byte) string {
	if len(body) > maxErrBody {
		body = body[:maxErrBody]
	}
	return string(bytes.TrimSpace(body))
}
