# Альфа-Хак · Новостная аналитика для принятия бизнес-решений

Сервис превращает поток внешних новостей и аналитику Альфа-Банка
в персонализированные бизнес-инсайты для ЛПР крупного бизнеса.

Это **рабочий README для команды**. Полная версия для сдачи (назначение,
архитектурная схема, пошаговый сценарий проверки, THIRD_PARTY_NOTICES)
собирается на Д5–Д8, владелец — Go #1 и Java (см. [docs/PROGRESS.md](docs/PROGRESS.md)).

- Задание кейса (первоисточник) — [docs/CASE.pdf](docs/CASE.pdf), текстом — [docs/CASE.txt](docs/CASE.txt)
- Что строим и кто за что отвечает — [docs/TEAM_PLAN.md](docs/TEAM_PLAN.md)
- Статус работ по участникам — [docs/PROGRESS.md](docs/PROGRESS.md)
- Архитектурные правила репозитория — [CLAUDE.md](CLAUDE.md)

---

## Быстрый старт

Нужны только Docker и Docker Compose. Сначала данные кейса (один раз, ~150 МБ):

```bash
make data
```

Потом весь стек:

```bash
docker compose -f deployments/docker-compose.yaml up --build
```

Вместе со стеком один раз отработает `research-loader`: распознаёт 9 PDF аналитики
ЦМИ и кладёт чанки в базу. Первый прогон — несколько минут (это сканы, нужен OCR),
дальше он пропускает уже загруженное. Отдельно его можно запустить так:

```bash
make research
```

Только Go-контур и база (быстрее, без сборки Java и Python):

```bash
make up-go
```

Проверить, что всё живо:

```bash
make smoke
```

| Сервис | Порт | Владелец | Состояние |
|---|---|---|---|
| `go-ingest` — парсер, оркестратор, SSE | 8080 | Go #1 | парсер и SSE работают |
| `java-agent` — агентное ядро, Hybrid RAG | 8081 | Java | скелет, эндпоинты 501 |
| `py-ml` — эмбеддинги, шум, сюжеты | 8000 | Python | скелет, эндпоинты 501 |
| `go-api` — Client Context Engine, BFF | 8090 | Go #2 | скелет, эндпоинты 501 |
| `llm-gateway` — единственный выход к моделям | 8091 | Go #1 | работает (нужен ключ провайдера) |
| `postgres` 16 + pgvector | **15432** на хосте (в сети compose — 5432) | Go #1 | 26 таблиц, миграции отдельным job |

## Что уже можно делать

```bash
# что собрал парсер
curl localhost:8080/metrics

# вбросить свою новость — живой путь для демо
curl -X POST localhost:8080/v1/news -H 'Content-Type: application/json' -d '{
  "url": "https://example.com/news/1",
  "title": "РЖД индексирует тарифы на восточном полигоне",
  "body": "Тариф на перевозку неугольных грузов индексируется на 10 процентов с 1 декабря."
}'

# live-панель пайплайна (Server-Sent Events)
curl -N localhost:8080/v1/pipeline/stream

# allowlist моделей и дневной расход
curl localhost:8091/v1/models
curl localhost:8091/v1/budget
```

Контракты: [api/openapi/ingest.yaml](api/openapi/ingest.yaml),
[api/openapi/gateway.yaml](api/openapi/gateway.yaml).

## Конфигурация

Все переменные перечислены в [.env.example](.env.example) — он же подключается
в compose как база значений. Свои значения и секреты кладите в `.env`
(он в `.gitignore` и не попадает в репозиторий).

Ключ к моделям нужен только `llm-gateway`:

```dotenv
LLM_UPSTREAM_BASE_URL=https://<адрес-провайдера>/v1
LLM_API_KEY=<ключ хакатона>
```

Без ключа стек поднимается, но `POST /v1/complete` отвечает `UPSTREAM_FAILED` —
так остальные стеки пишутся против живого контракта с первого дня.

Allowlist моделей, маршрутизация задач и цены — [configs/llm.yaml](configs/llm.yaml).
Модель вне этого файла gateway не вызовет (Ограничение 04 кейса).

## Данные кейса

Файлы датасета и аналитики в репозиторий не коммитятся (150 МБ, см. `.gitignore`).
Скачиваются одной командой по ссылкам из ТЗ:

```bash
bash scripts/fetch_case_data.sh
```

```
data/
  df_Uid_data_hack.csv       юрлица, 85 525 строк
  df_Uid_payment_hack.csv    граф платежей, 1 380 585 строк
  df_Fid_Uid_link_hack.csv   связи ФЛ и ЮЛ, 12 514 строк
  research/                  9 материалов ЦМИ Альфа-Банка, 64 страницы (сканы, нужен OCR)
```

Что важно знать перед загрузкой данных — [data/README.md](data/README.md).

## Разработка

```bash
make build        # собрать три Go-сервиса
make test-race    # юнит-тесты с -race
make lint         # golangci-lint
make migrate-up   # применить миграции к локальной базе
```

Перед тем как считать задачу сделанной: `make lint` чистый и `make test-race` зелёный.

Правила кода, слои и границы — [CLAUDE.md](CLAUDE.md). Главное:
одна ветка на сервис, PR в `main`, обращения к моделям только через `llm-gateway`,
ни один вывод не идёт в UI без `evidence` и `attribution`.

## Структура репозитория

Каждый сервис — своя папка в `services/`, у Go-сервисов внутри свой `internal/`:
компилятор не даст одному сервису залезть в потроха другого. Общий код —
в корневом `internal/`.

```
api/openapi/              контракты REST (пишутся руками, держатся в синхроне с кодом)
configs/llm.yaml          allowlist моделей, маршрутизация задач, цены
deployments/              docker-compose и Dockerfile'ы
docs/                     задание кейса, план команды, трекер прогресса
internal/                 ОБЩЕЕ для всех Go-сервисов
  domain/                 сущности, sentinel-ошибки, дедуп, статусная машина
  config/                 HTTPConfig, PostgresConfig, Parse
  infra/db/postgres/      пул, транзакции, хелперы ошибок, миграции
  transport/http/         http.Server, apierrors, middleware
  usecase/common/         TxManager
services/
  go-ingest/              Go #1: парсер лент, дедуп, оркестратор, SSE
    cmd/                  точка входа
    internal/             app, config, usecase, repository, infra, transport, worker
  llm-gateway/            Go #1: allowlist моделей, кэш, бюджет, аудит
  go-api/                 Go #2: Client Context Engine, ранжирование, BFF
  java-agent/             Java: агентное ядро и Hybrid RAG
  py-ml/                  Python: эмбеддинги, шум, сюжеты
scripts/                  smoke.sh и загрузка данных кейса
```

Кто какие файлы правит — [CLAUDE.md §13](CLAUDE.md).
