package ru.alfahack.agent.repository;

import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.UUID;

import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.stereotype.Repository;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import ru.alfahack.agent.cib.RiskTag;
import ru.alfahack.agent.rag.ResearchChunkStore;
import ru.alfahack.agent.rag.ScoredChunk;

@Repository
public class JdbcResearchChunkStore implements ResearchChunkStore {

    private static final String LEXICAL_SQL = """
            SELECT c.id, d.title AS document, c.page, c.text,
                   word_similarity(c.text, :query) AS score, c.entities
            FROM research_chunks c
            JOIN research_documents d ON d.id = c.document_id
            ORDER BY score DESC, c.id
            LIMIT :limit
            """;

    private static final String VECTOR_SQL = """
            SELECT c.id, d.title AS document, c.page, c.text,
                   1 - (c.embedding <=> :vec::vector) AS score, c.entities
            FROM research_chunks c
            JOIN research_documents d ON d.id = c.document_id
            WHERE c.embedding IS NOT NULL
            ORDER BY c.embedding <=> :vec::vector, c.id
            LIMIT :limit
            """;

    private final NamedParameterJdbcTemplate jdbc;
    private final ObjectMapper mapper;

    public JdbcResearchChunkStore(NamedParameterJdbcTemplate jdbc, ObjectMapper mapper) {
        this.jdbc = jdbc;
        this.mapper = mapper;
    }

    @Override
    public List<ScoredChunk> searchLexical(String query, int limit) {
        return jdbc.query(LEXICAL_SQL,
                new MapSqlParameterSource()
                        .addValue("query", query)
                        .addValue("limit", limit),
                (rs, i) -> toChunk(rs));
    }

    @Override
    public List<ScoredChunk> searchVector(float[] embedding, int limit) {
        return jdbc.query(VECTOR_SQL,
                new MapSqlParameterSource()
                        .addValue("vec", vectorLiteral(embedding))
                        .addValue("limit", limit),
                (rs, i) -> toChunk(rs));
    }

    @Override
    public boolean hasEmbeddings() {
        Boolean exists = jdbc.queryForObject(
                "SELECT EXISTS (SELECT 1 FROM research_chunks WHERE embedding IS NOT NULL)",
                new MapSqlParameterSource(), Boolean.class);
        return Boolean.TRUE.equals(exists);
    }

    private ScoredChunk toChunk(ResultSet rs) throws SQLException {
        return new ScoredChunk(
                rs.getObject("id", UUID.class),
                rs.getString("document"),
                rs.getInt("page"),
                rs.getString("text"),
                rs.getDouble("score"),
                false,
                false,
                riskTags(rs.getString("entities")));
    }

    private Set<RiskTag> riskTags(String entitiesJson) {
        if (entitiesJson == null || entitiesJson.isBlank()) {
            return Set.of();
        }
        try {
            JsonNode root = mapper.readTree(entitiesJson);
            Set<RiskTag> tags = new HashSet<>();
            for (JsonNode entity : root) {
                if ("RISK_TAG".equals(entity.path("type").asText())) {
                    String value = entity.path("value").asText();
                    RiskTag.parseLenient(value).ifPresent(tags::add);
                }
            }
            return tags;
        } catch (Exception e) {
            return Set.of();
        }
    }

    private static String vectorLiteral(float[] embedding) {
        StringBuilder sb = new StringBuilder(embedding.length * 10);
        for (int i = 0; i < embedding.length; i++) {
            if (i > 0) {
                sb.append(',');
            }
            sb.append(embedding[i]);
        }
        return "[" + sb + "]";
    }
}
