DROP INDEX IF EXISTS research_chunks_entities_idx;
ALTER TABLE research_chunks DROP COLUMN IF EXISTS entities, DROP COLUMN IF EXISTS okved_hints;
