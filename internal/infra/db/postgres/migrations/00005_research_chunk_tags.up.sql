-- 00005_research_chunk_tags — классификация чанков аналитики ЦМИ.
--
-- ТЗ требует от парсера классификацию не только новостей, но и аналитических
-- материалов Альфа-Банка. Чанк размечается тем же словарём, что и новость
-- (configs/enrichment.yaml): отрасли ОКВЭД, страны, сырьё, риск-теги. Форма JSON —
-- та же, что в news_enrichment, поэтому агент сопоставляет их напрямую:
--   SELECT ... FROM research_chunks
--   WHERE entities @> '[{"type": "RISK_TAG", "value": "rate_risk"}]';
ALTER TABLE research_chunks
    ADD COLUMN okved_hints jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN entities    jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX research_chunks_entities_idx ON research_chunks USING gin (entities jsonb_path_ops);
