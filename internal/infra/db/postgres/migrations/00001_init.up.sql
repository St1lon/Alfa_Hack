-- 00001_init — схема данных Альфа-Хак.
--
-- Владение таблицами (см. docs/TEAM_PLAN.md §4):
--   Go #1   — news_sources, news, stories, story_news, research_*, pipeline_events,
--             job_attempts, dead_letters, llm_*
--   Go #2   — companies, company_persons, counterparty_edges, company_features,
--             company_overrides, story_company_scores, feedback
--   Java    — insights, evidence, agent_runs
--   Python  — news_enrichment
-- Каждый владелец добавляет свои колонки НОВОЙ миграцией, а не правкой этой.
--
-- Допущения, которые надо подтвердить (docs/PROGRESS.md, раздел «Открытые вопросы»):
--   * размерность эмбеддинга — 768 (multilingual-e5-base / rubert-base);
--   * деньги LLM считаем в копейках (bigint), как и везде в проекте.

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ───────────────────────── Справочники ─────────────────────────
-- Нормализованы таблицами, а не CHECK/ENUM (CLAUDE.md §5).

CREATE TABLE pipeline_statuses (
    id    smallint PRIMARY KEY,
    code  text NOT NULL UNIQUE,
    title text NOT NULL
);

INSERT INTO pipeline_statuses (id, code, title) VALUES
    (1, 'RAW',       'Сырая новость получена парсером'),
    (2, 'DEDUPED',   'Прошла дедупликацию'),
    (3, 'ENRICHED',  'Обогащена эмбеддингом и сущностями'),
    (4, 'SCORED',    'Подобраны компании по признакам'),
    (5, 'ANALYZED',  'Агент построил разбор'),
    (6, 'PUBLISHED', 'Инсайт виден в ленте'),
    (7, 'FAILED',    'Шаг упал, ушла в dead_letters');

CREATE TABLE attribution_types (
    id    smallint PRIMARY KEY,
    code  text NOT NULL UNIQUE,
    title text NOT NULL
);

INSERT INTO attribution_types (id, code, title) VALUES
    (1, 'FACT',         'Подтверждено первоисточником'),
    (2, 'BANK_VIEW',    'Позиция Альфа-Банка из материалов кейса'),
    (3, 'AI_INFERENCE', 'Вывод модели');

-- ───────────────────────── Новости (Go #1) ─────────────────────────

CREATE TABLE news_sources (
    id                smallserial PRIMARY KEY,
    code              text NOT NULL UNIQUE,
    title             text NOT NULL,
    kind              text NOT NULL,              -- 'rss' | 'html'
    url               text NOT NULL,
    enabled           boolean NOT NULL DEFAULT true,
    poll_interval_sec integer NOT NULL DEFAULT 300,
    last_polled_at    timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE stories (
    id                 uuid PRIMARY KEY,
    title              text NOT NULL,
    summary            text,
    first_published_at timestamptz,
    last_published_at  timestamptz,
    sources_count      integer NOT NULL DEFAULT 1,
    status_id          smallint NOT NULL REFERENCES pipeline_statuses (id),
    embedding          vector(768),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX stories_status_idx ON stories (status_id, created_at DESC);

CREATE TABLE news (
    id            uuid PRIMARY KEY,
    source_id     smallint REFERENCES news_sources (id),
    url           text,
    canonical_url text,
    url_hash      char(64),                       -- sha256 канонизированного URL
    text_hash     char(64) NOT NULL,              -- sha256 нормализованного текста
    title         text NOT NULL,
    body          text NOT NULL,
    publisher     text,
    lang          text NOT NULL DEFAULT 'ru',
    published_at  timestamptz,
    fetched_at    timestamptz NOT NULL DEFAULT now(),
    status_id     smallint NOT NULL REFERENCES pipeline_statuses (id),
    story_id      uuid REFERENCES stories (id) ON DELETE SET NULL,
    raw           jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX news_url_hash_uidx  ON news (url_hash) WHERE url_hash IS NOT NULL;
CREATE UNIQUE INDEX news_text_hash_uidx ON news (text_hash);
CREATE INDEX news_status_idx     ON news (status_id, fetched_at);
CREATE INDEX news_story_idx      ON news (story_id);
CREATE INDEX news_published_idx  ON news (published_at DESC);
CREATE INDEX news_title_trgm_idx ON news USING gin (title gin_trgm_ops);

-- Обогащение новости (пишет py-ml; при USE_ML_FALLBACK=true — go-ingest).
CREATE TABLE news_enrichment (
    news_id       uuid PRIMARY KEY REFERENCES news (id) ON DELETE CASCADE,
    embedding     vector(768),
    entities      jsonb NOT NULL DEFAULT '[]'::jsonb,
    okved_hints   jsonb NOT NULL DEFAULT '[]'::jsonb,
    noise_score   real,
    model_version text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE story_news (
    story_id   uuid NOT NULL REFERENCES stories (id) ON DELETE CASCADE,
    news_id    uuid NOT NULL REFERENCES news (id) ON DELETE CASCADE,
    is_primary boolean NOT NULL DEFAULT false,
    PRIMARY KEY (story_id, news_id)
);

-- ───────────────────────── Аналитика Альфа-Банка (Go #1) ─────────────────────────

CREATE TABLE research_documents (
    id           smallserial PRIMARY KEY,
    code         text NOT NULL UNIQUE,
    title        text NOT NULL,
    file_name    text NOT NULL,
    published_at date,
    pages        integer,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE research_chunks (
    id          uuid PRIMARY KEY,
    document_id smallint NOT NULL REFERENCES research_documents (id) ON DELETE CASCADE,
    page        integer NOT NULL,                -- обязателен: на нём держится провенанс
    chunk_index integer NOT NULL,
    text        text NOT NULL,
    tokens      integer,
    embedding   vector(768),
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (document_id, chunk_index)
);

CREATE INDEX research_chunks_text_trgm_idx ON research_chunks USING gin (text gin_trgm_ops);

-- ───────────────────────── Компании (Go #2) ─────────────────────────

CREATE TABLE companies (
    company_id                     text PRIMARY KEY,         -- SYNTHETIC_U_...
    org_type                       text,
    okfs_name                      text,
    company_age_years_gradation    text,
    eco_mark_label                 text,
    mainokved_code                 text,
    mainokved_name                 text,
    amount_workers_fns_gradation   text,
    amount_workers_spark_gradation text,
    cl_type                        text,
    revenue_company                numeric(20, 2),
    group_id                       text,
    city                           text,
    region                         text,
    negativemark                   boolean,
    fts_flag                       boolean,
    fts_country_name               text,
    cl_product_rko_mark            boolean,
    cl_product_gar_mark            boolean,
    cl_product_loa_mark            boolean,
    cl_product_lea_mark            boolean,
    cl_product_dep_mark            boolean,
    cl_product_zpp_mark            boolean,
    factoring_mark                 boolean,
    display_alias                  text,                     -- псевдоним для демо, помечен как синтетический
    created_at                     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX companies_okved_idx  ON companies (mainokved_code);
CREATE INDEX companies_region_idx ON companies (region);
CREATE INDEX companies_group_idx  ON companies (group_id);
CREATE INDEX companies_fts_idx    ON companies (fts_flag, fts_country_name);

CREATE TABLE company_persons (
    id         bigserial PRIMARY KEY,
    company_id text NOT NULL REFERENCES companies (company_id) ON DELETE CASCADE,
    person_id  text NOT NULL,                                -- SYNTHETIC_F_...
    city       text,
    region     text
);

CREATE INDEX company_persons_company_idx ON company_persons (company_id);
CREATE INDEX company_persons_person_idx  ON company_persons (person_id);

CREATE TABLE counterparty_edges (
    id          bigserial PRIMARY KEY,
    payer       text NOT NULL,
    beneficiary text NOT NULL,
    pay_amt     numeric(20, 2) NOT NULL DEFAULT 0,
    pay_cnt     integer NOT NULL DEFAULT 0
);

CREATE INDEX counterparty_payer_idx       ON counterparty_edges (payer);
CREATE INDEX counterparty_beneficiary_idx ON counterparty_edges (beneficiary);

-- Выведенные признаки компании; derived_from обязателен — это основа критерия 12.
CREATE TABLE company_features (
    id            bigserial PRIMARY KEY,
    company_id    text NOT NULL REFERENCES companies (company_id) ON DELETE CASCADE,
    feature_key   text NOT NULL,
    feature_value jsonb NOT NULL,
    derived_from  text NOT NULL,
    computed_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, feature_key)
);

CREATE TABLE company_overrides (
    id            bigserial PRIMARY KEY,
    company_id    text NOT NULL REFERENCES companies (company_id) ON DELETE CASCADE,
    feature_key   text NOT NULL,
    feature_value jsonb NOT NULL,
    author        text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, feature_key)
);

-- Результат ранжирования «сюжет — компания» (Go #2). Порог отсечения — в конфиге сервиса.
CREATE TABLE story_company_scores (
    id              bigserial PRIMARY KEY,
    story_id        uuid NOT NULL REFERENCES stories (id) ON DELETE CASCADE,
    company_id      text NOT NULL REFERENCES companies (company_id) ON DELETE CASCADE,
    relevance_score real NOT NULL,
    match_reasons   jsonb NOT NULL DEFAULT '[]'::jsonb,
    model_version   text NOT NULL,
    scored_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (story_id, company_id, model_version)
);

CREATE INDEX story_company_scores_story_idx ON story_company_scores (story_id, relevance_score DESC);

-- ───────────────────────── Инсайты и агент (Java) ─────────────────────────

CREATE TABLE agent_runs (
    id                uuid PRIMARY KEY,
    story_id          uuid REFERENCES stories (id) ON DELETE SET NULL,
    company_id        text REFERENCES companies (company_id) ON DELETE SET NULL,
    status            text NOT NULL,
    steps_count       integer NOT NULL DEFAULT 0,
    critic_iterations integer NOT NULL DEFAULT 0,
    tokens_used       integer NOT NULL DEFAULT 0,
    cost_kop          bigint NOT NULL DEFAULT 0,
    trace             jsonb NOT NULL DEFAULT '[]'::jsonb,
    started_at        timestamptz NOT NULL DEFAULT now(),
    finished_at       timestamptz
);

CREATE TABLE insights (
    insight_id      uuid PRIMARY KEY,
    story_id        uuid NOT NULL REFERENCES stories (id) ON DELETE CASCADE,
    company_id      text NOT NULL REFERENCES companies (company_id) ON DELETE CASCADE,
    agent_run_id    uuid REFERENCES agent_runs (id) ON DELETE SET NULL,
    model_version   text NOT NULL,
    confidence      text NOT NULL,
    relevance_score real,
    impact_score    smallint,
    severity        text,
    payload         jsonb NOT NULL,              -- полный контракт инсайта, TEAM_PLAN §10
    generated_at    timestamptz NOT NULL DEFAULT now(),
    published_at    timestamptz,
    -- идемпотентность пайплайна: один разбор на пару «сюжет — компания» и версию модели
    UNIQUE (story_id, company_id, model_version)
);

CREATE INDEX insights_company_idx ON insights (company_id, generated_at DESC);
CREATE INDEX insights_story_idx   ON insights (story_id);

CREATE TABLE evidence (
    id             bigserial PRIMARY KEY,
    insight_id     uuid NOT NULL REFERENCES insights (insight_id) ON DELETE CASCADE,
    field_path     text NOT NULL,               -- например impact_analysis.second_order_company_effect
    ref_type       text NOT NULL,               -- 'news' | 'research_chunk' | 'company_feature'
    ref            text NOT NULL,
    attribution_id smallint NOT NULL REFERENCES attribution_types (id)
);

CREATE INDEX evidence_insight_idx ON evidence (insight_id);

CREATE TABLE feedback (
    id         bigserial PRIMARY KEY,
    insight_id uuid NOT NULL REFERENCES insights (insight_id) ON DELETE CASCADE,
    rating     smallint NOT NULL,
    tags       text[] NOT NULL DEFAULT '{}',
    comment    text,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- ───────────────────────── Пайплайн и LLM-Gateway (Go #1) ─────────────────────────

-- Лента событий пайплайна: источник SSE-стрима для live-панели на демо.
CREATE TABLE pipeline_events (
    id          bigserial PRIMARY KEY,
    entity_kind text NOT NULL,                   -- 'news' | 'story' | 'insight'
    entity_id   text NOT NULL,
    status_id   smallint REFERENCES pipeline_statuses (id),
    service     text NOT NULL,
    message     text,
    payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX pipeline_events_created_idx ON pipeline_events (id DESC);

-- Ретраи шагов оркестратора.
CREATE TABLE job_attempts (
    id          bigserial PRIMARY KEY,
    entity_kind text NOT NULL,
    entity_id   text NOT NULL,
    stage       text NOT NULL,
    attempts    integer NOT NULL DEFAULT 0,
    next_run_at timestamptz NOT NULL DEFAULT now(),
    last_error  text,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (entity_kind, entity_id, stage)
);

CREATE INDEX job_attempts_due_idx ON job_attempts (stage, next_run_at);

CREATE TABLE dead_letters (
    id          bigserial PRIMARY KEY,
    entity_kind text NOT NULL,
    entity_id   text NOT NULL,
    stage       text NOT NULL,
    attempts    integer NOT NULL,
    last_error  text,
    payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Аудит всех вызовов моделей. Единственная точка выхода наружу — llm-gateway.
CREATE TABLE llm_calls (
    id                uuid PRIMARY KEY,
    service           text NOT NULL,             -- go-ingest | go-api | java-agent | py-ml
    actor             text,                      -- кто из команды гонял запрос
    task              text NOT NULL,             -- classify | analyze | critic | ...
    model             text NOT NULL,
    prompt_hash       char(64) NOT NULL,
    cache_hit         boolean NOT NULL DEFAULT false,
    prompt_tokens     integer NOT NULL DEFAULT 0,
    completion_tokens integer NOT NULL DEFAULT 0,
    total_tokens      integer NOT NULL DEFAULT 0,
    cost_kop          bigint NOT NULL DEFAULT 0,
    latency_ms        integer NOT NULL DEFAULT 0,
    status            text NOT NULL,             -- ok | upstream_error | budget_exceeded | model_denied
    error             text,
    request_id        text,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX llm_calls_created_idx ON llm_calls (created_at DESC);
CREATE INDEX llm_calls_service_idx ON llm_calls (service, created_at DESC);

-- Кэш ответов по хэшу промпта: переживает перезапуски и спасает демо.
CREATE TABLE llm_cache (
    prompt_hash  char(64) PRIMARY KEY,
    model        text NOT NULL,
    response     jsonb NOT NULL,
    total_tokens integer NOT NULL DEFAULT 0,
    hits         integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_hit_at  timestamptz
);

-- Дневной счётчик бюджета для soft- и hard-лимита.
CREATE TABLE llm_budget_days (
    day        date PRIMARY KEY,
    spent_kop  bigint NOT NULL DEFAULT 0,
    tokens     bigint NOT NULL DEFAULT 0,
    calls      integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);
