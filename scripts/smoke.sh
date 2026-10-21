#!/usr/bin/env bash
# Быстрая проверка живого контура Go #1: health, парсер, вброс новости, дедуп,
# SSE и эндпоинты gateway. Запуск: make smoke (стек должен быть поднят).
set -euo pipefail

INGEST="${INGEST_URL:-http://localhost:8080}"
GATEWAY="${GATEWAY_URL:-http://localhost:8091}"
API="${API_URL:-http://localhost:8090}"

say() { printf '\n=== %s ===\n' "$1"; }

say "health всех трёх Go-сервисов"
curl -fsS "$INGEST/readyz"; echo
curl -fsS "$GATEWAY/readyz"; echo
curl -fsS "$API/readyz"; echo

say "сколько новостей успел собрать парсер"
curl -fsS "$INGEST/metrics"; echo

# Тело — через stdin, а не аргументом -d: на Windows curl.exe получает аргументы
# в ANSI-кодировке, и кириллица превращается в символы-заменители ещё до сервера.
post_news() {
  printf '%s' "$PAYLOAD" | curl -fsS -X POST "$INGEST/v1/news" \
    -H 'Content-Type: application/json; charset=utf-8' --data-binary @-
}

say "вброс новости (уникальный текст)"
STAMP="$(date +%s)"
PAYLOAD=$(cat <<JSON
{
  "url": "https://example.com/smoke/$STAMP",
  "title": "Смоук-тест пайплайна $STAMP",
  "body": "Проверочная новость смоук-теста: ключевая ставка, экспортные пошлины и тарифы РЖД. Прогон $STAMP.",
  "publisher": "smoke-test"
}
JSON
)
post_news; echo

say "повторный вброс того же текста — ожидаем duplicate:true"
post_news; echo

say "последние события пайплайна по SSE (2 секунды)"
curl -fsS -N --max-time 2 "$INGEST/v1/pipeline/stream?after=0" | head -20 || true

say "allowlist моделей и дневной бюджет"
curl -fsS "$GATEWAY/v1/models"; echo
curl -fsS "$GATEWAY/v1/budget"; echo

say "модель вне allowlist должна быть отбита (MODEL_NOT_ALLOWED)"
curl -s -o /dev/null -w 'HTTP %{http_code}\n' -X POST "$GATEWAY/v1/complete" \
  -H 'Content-Type: application/json' \
  -d '{"service":"smoke","task":"analyze","model":"gpt-4o","messages":[{"role":"user","content":"ping"}]}'

printf '\nСмоук-тест прошёл.\n'
