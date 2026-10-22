package ingesthttp

import (
	_ "embed"
	"net/http"
)

// panelHTML — live-панель пайплайна: воронка, журнал событий и вброс новости.
// Без внешних зависимостей, чтобы фронт Go #2 мог встроить её как есть.
//
//go:embed panel.html
var panelHTML []byte

// Panel отдаёт live-панель пайплайна: GET /panel.
func (h *Handlers) Panel(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(panelHTML); err != nil {
		h.deps.Log.Debug("write panel failed")
	}
}
