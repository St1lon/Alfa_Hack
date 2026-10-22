package ru.alfahack.agent.repository;

import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.UUID;

import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.stereotype.Repository;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.cib.RiskTag;

@Repository
public class StoryRepository {

    private final NamedParameterJdbcTemplate jdbc;
    private final ObjectMapper mapper;

    public StoryRepository(NamedParameterJdbcTemplate jdbc, ObjectMapper mapper) {
        this.jdbc = jdbc;
        this.mapper = mapper;
    }

    public AgentRunContext.StoryBundle loadBundle(UUID storyId) {
        var story = jdbc.queryForObject("""
                SELECT title, summary FROM stories WHERE id = :id
                """, new MapSqlParameterSource("id", storyId), (rs, i) -> new Object[] {
                rs.getString("title"), rs.getString("summary") });
        List<AgentRunContext.StoryBundle.Source> sources = jdbc.query("""
                SELECT n.id, n.publisher, n.url, n.published_at, sn.is_primary
                FROM story_news sn JOIN news n ON n.id = sn.news_id
                WHERE sn.story_id = :id
                ORDER BY sn.is_primary DESC, n.published_at
                """, new MapSqlParameterSource("id", storyId), StoryRepository::toSource);

        List<Object[]> enrichment = jdbc.query("""
                SELECT e.entities FROM news_enrichment e
                JOIN story_news sn ON sn.news_id = e.news_id
                WHERE sn.story_id = :id AND sn.is_primary
                """, new MapSqlParameterSource("id", storyId), (rs, i) -> new Object[] { rs.getString("entities") });

        Set<RiskTag> tags = new HashSet<>();
        List<String> countries = new ArrayList<>();
        for (Object[] row : enrichment) {
            parseEntities((String) row[0], tags, countries);
        }
        return new AgentRunContext.StoryBundle(storyId, (String) story[0], (String) story[1], sources,
                List.copyOf(tags), countries);
    }

    public List<AgentRunContext.SimilarStory> findSimilar(UUID storyId, String title, int limit) {
        return jdbc.query("""
                SELECT id, title, first_published_at FROM stories
                WHERE id <> :id
                ORDER BY word_similarity(title, :title) DESC, first_published_at DESC
                LIMIT :limit
                """, new MapSqlParameterSource()
                .addValue("id", storyId)
                .addValue("title", title)
                .addValue("limit", limit), (rs, i) -> new AgentRunContext.SimilarStory(
                rs.getObject("id", UUID.class), rs.getString("title"),
                rs.getObject("first_published_at", java.time.OffsetDateTime.class)));
    }

    public ru.alfahack.agent.domain.Insight.Personalization loadPersonalization(UUID storyId, String companyId) {
        List<String> raw = jdbc.query("""
                SELECT relevance_score::text, match_reasons::text FROM story_company_scores
                WHERE story_id = :story AND company_id = :company
                ORDER BY scored_at DESC LIMIT 1
                """, new MapSqlParameterSource()
                .addValue("story", storyId)
                .addValue("company", companyId),
                (rs, i) -> rs.getString(1) + "\n" + rs.getString(2));
        if (raw.isEmpty()) {
            return new ru.alfahack.agent.domain.Insight.Personalization(null, List.of(), List.of());
        }
        String[] parts = raw.get(0).split("\n", 2);
        Double relevance = null;
        try {
            relevance = Double.valueOf(parts[0]);
        } catch (NumberFormatException ignored) {
        }
        List<ru.alfahack.agent.domain.Insight.MatchReason> reasons = new ArrayList<>();
        try {
            JsonNode root = mapper.readTree(parts[1]);
            for (JsonNode node : root) {
                Double weight = node.hasNonNull("weight") ? node.get("weight").asDouble() : null;
                reasons.add(new ru.alfahack.agent.domain.Insight.MatchReason(
                        node.path("feature").asText(null), node.path("label").asText(null), weight));
            }
        } catch (Exception ignored) {
        }
        return new ru.alfahack.agent.domain.Insight.Personalization(relevance, reasons, List.of());
    }

    private void parseEntities(String entitiesJson, Set<RiskTag> tags, List<String> countries) {
        if (entitiesJson == null || entitiesJson.isBlank()) {
            return;
        }
        try {
            JsonNode root = mapper.readTree(entitiesJson);
            for (JsonNode entity : root) {
                String type = entity.path("type").asText();
                String value = entity.path("value").asText();
                if ("RISK_TAG".equals(type)) {
                    RiskTag.parseLenient(value).ifPresent(tags::add);
                } else if ("COUNTRY".equals(type) && !value.isBlank() && countries.size() < 5) {
                    countries.add(value);
                }
            }
        } catch (Exception ignored) {
        }
    }

    private static AgentRunContext.StoryBundle.Source toSource(ResultSet rs, int i) throws SQLException {
        return new AgentRunContext.StoryBundle.Source(
                rs.getObject("id", UUID.class),
                rs.getString("publisher"),
                rs.getString("url"),
                rs.getObject("published_at", java.time.OffsetDateTime.class),
                rs.getBoolean("is_primary"));
    }
}
