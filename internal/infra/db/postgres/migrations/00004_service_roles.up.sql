-- 00004_service_roles — отдельная роль БД на каждый сервис (Ограничение 06 кейса:
-- разграничение прав доступа). Каждый сервис видит только то, что ему нужно.
--
-- Главное, что это даёт: у llm-gateway — единственного сервиса с ключом к моделям —
-- нет доступа к данным клиентов вообще, только к трём своим таблицам. Утечь через
-- него в модель данные компаний не могут даже по ошибке в коде.
--
-- Роли создаются без права входа (NOLOGIN) и без паролей: пароли не место в SQL
-- репозитория. Вход и пароль из окружения включает job db-roles в compose
-- (deployments/db-roles.sh) сразу после миграций.
--
-- Новая таблица в вашем диапазоне миграций → GRANT своей роли в той же миграции.

DO $$
DECLARE
    role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY['svc_ingest', 'svc_gateway', 'svc_api', 'svc_agent', 'svc_ml'] LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('CREATE ROLE %I NOLOGIN', role_name);
        END IF;
    END LOOP;
END
$$;

GRANT USAGE ON SCHEMA public TO svc_ingest, svc_gateway, svc_api, svc_agent, svc_ml;

-- Справочники читают все, кто работает со статусами и атрибуцией.
GRANT SELECT ON pipeline_statuses, attribution_types TO svc_ingest, svc_api, svc_agent, svc_ml;

-- ───────────────────────── go-ingest (и его research-loader) ─────────────────────────
GRANT SELECT, UPDATE                 ON news_sources                    TO svc_ingest;
GRANT SELECT, INSERT, UPDATE         ON news, news_enrichment, stories  TO svc_ingest;
GRANT SELECT, INSERT                 ON story_news                      TO svc_ingest;
GRANT SELECT, INSERT, UPDATE         ON research_documents              TO svc_ingest;
GRANT SELECT, INSERT, DELETE         ON research_chunks                 TO svc_ingest;
GRANT SELECT, INSERT                 ON pipeline_events                 TO svc_ingest;
GRANT SELECT, INSERT, UPDATE, DELETE ON job_attempts                    TO svc_ingest;
GRANT INSERT                         ON dead_letters                    TO svc_ingest;
-- читает, каким компаниям релевантен сюжет, чтобы отдать агенту топ-N
GRANT SELECT                         ON story_company_scores            TO svc_ingest;
GRANT USAGE ON SEQUENCE research_documents_id_seq, pipeline_events_id_seq,
                        job_attempts_id_seq, dead_letters_id_seq       TO svc_ingest;

-- ───────────────────────── llm-gateway: только свои таблицы ─────────────────────────
GRANT SELECT, INSERT                 ON llm_calls                       TO svc_gateway;
GRANT SELECT, INSERT, UPDATE         ON llm_cache, llm_budget_days      TO svc_gateway;

-- ───────────────────────── go-api (Go #2) ─────────────────────────
GRANT SELECT, INSERT, UPDATE, DELETE ON companies, company_persons, counterparty_edges,
                                        company_features, company_overrides,
                                        story_company_scores, feedback  TO svc_api;
GRANT SELECT ON news, news_enrichment, stories, story_news,
                research_documents, research_chunks,
                insights, evidence, agent_runs, pipeline_events        TO svc_api;
GRANT USAGE ON SEQUENCE company_persons_id_seq, counterparty_edges_id_seq,
                        company_features_id_seq, company_overrides_id_seq,
                        story_company_scores_id_seq, feedback_id_seq    TO svc_api;

-- ───────────────────────── java-agent ─────────────────────────
GRANT SELECT, INSERT, UPDATE, DELETE ON insights, evidence, agent_runs  TO svc_agent;
GRANT SELECT ON news, news_enrichment, stories, story_news,
                research_documents, research_chunks,
                companies, company_persons, counterparty_edges,
                company_features, company_overrides, story_company_scores TO svc_agent;
-- шаги агента — в live-панель пайплайна
GRANT INSERT ON pipeline_events                                        TO svc_agent;
GRANT USAGE ON SEQUENCE evidence_id_seq, pipeline_events_id_seq        TO svc_agent;

-- ───────────────────────── py-ml: только чтение новостей ─────────────────────────
-- py-ml отвечает на запросы оркестратора, в базу не пишет (docs/PIPELINE_CONTRACTS.md).
GRANT SELECT ON news, news_enrichment, stories, story_news             TO svc_ml;
