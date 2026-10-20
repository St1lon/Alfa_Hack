package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/st1lon/alfa-hack/internal/transport/http/apierrors"
)

// ServiceToken проверяет внутренний токен сервисов в заголовке Authorization.
// Пустой token выключает проверку — так работает локальная разработка;
// на внешнем сервере токен задаётся обязательно (Ограничение 06).
func ServiceToken(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(apierrors.Unauthorized())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
