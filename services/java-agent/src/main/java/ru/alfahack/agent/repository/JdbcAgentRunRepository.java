package ru.alfahack.agent.repository;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.stereotype.Repository;

import ru.alfahack.agent.agent.AgentRunRepository;
import ru.alfahack.agent.agent.EvidenceRow;
import ru.alfahack.agent.domain.Insight;

@Repository
public class JdbcAgentRunRepository implements AgentRunRepository {

    private final NamedParameterJdbcTemplate jdbc;
    private final com.fasterxml.jackson.databind.ObjectMapper mapper;

    public JdbcAgentRunRepository(NamedParameterJdbcTemplate jdbc, com.fasterxml.jackson.databind.ObjectMapper mapper) {
        this.jdbc = jdbc;
        this.mapper = mapper;
    }

    @Override
    public Optional<UUID> findInsightId(UUID storyId, String companyId, String modelVersion) {
        List<UUID> ids = jdbc.query("""
                SELECT insight_id FROM insights
                WHERE story_id = :story AND company_id = :company AND model_version = :version
                """, new MapSqlParameterSource()
                .addValue("story", storyId)
                .addValue("company", companyId)
                .addValue("version", modelVersion),
                (rs, i) -> rs.getObject("insight_id", UUID.class));
        return ids.stream().findFirst();
    }

    @Override
    public Optional<Insight> findInsight(UUID insightId) {
        List<String> payloads = jdbc.query("""
                SELECT payload::text FROM insights WHERE insight_id = :id
                """, new MapSqlParameterSource("id", insightId),
                (rs, i) -> rs.getString("payload"));
        if (payloads.isEmpty()) {
            return Optional.empty();
        }
        try {
            return Optional.of(mapper.readValue(payloads.get(0), Insight.class));
        } catch (Exception e) {
            throw new IllegalStateException("инсайт не читается: " + insightId, e);
        }
    }

    @Override
    public Optional<InsightCard> loadCard(UUID insightId) {
        Optional<Insight> insight = findInsight(insightId);
        if (insight.isEmpty()) {
            return Optional.empty();
        }
        List<EvidenceView> evidence = jdbc.query("""
                SELECT e.field_path, e.ref_type, e.ref, a.code
                FROM evidence e JOIN attribution_types a ON a.id = e.attribution_id
                WHERE e.insight_id = :id ORDER BY e.id
                """, new MapSqlParameterSource("id", insightId),
                (rs, i) -> new EvidenceView(rs.getString("field_path"), rs.getString("ref_type"),
                        rs.getString("ref"), rs.getString("code")));
        return Optional.of(new InsightCard(insight.get(), evidence));
    }

    @Override
    public Optional<RunTrace> loadTrace(UUID insightId) {
        List<RunTrace> traces = jdbc.query("""
                SELECT r.id, r.status, r.steps_count, r.critic_iterations, r.tokens_used,
                    r.cost_kop, r.trace::text AS trace
                FROM agent_runs r JOIN insights i ON i.agent_run_id = r.id
                WHERE i.insight_id = :id
                """, new MapSqlParameterSource("id", insightId),
                (rs, i) -> new RunTrace(rs.getObject("id", UUID.class), rs.getString("status"),
                        rs.getInt("steps_count"), rs.getInt("critic_iterations"),
                        rs.getInt("tokens_used"), rs.getLong("cost_kop"), rs.getString("trace")));
        return traces.stream().findFirst();
    }

    @Override
    public UUID startRun(UUID storyId, String companyId) {
        UUID runId = UUID.randomUUID();
        jdbc.update("""
                INSERT INTO agent_runs (id, story_id, company_id, status)
                VALUES (:id, :story, :company, 'RUNNING')
                """, new MapSqlParameterSource()
                .addValue("id", runId)
                .addValue("story", storyId)
                .addValue("company", companyId));
        return runId;
    }

    @Override
    public void finishRun(UUID runId, String status, int stepsCount, int criticIterations,
            int tokensUsed, long costKop, String traceJson) {
        jdbc.update("""
                UPDATE agent_runs SET status = :status, steps_count = :steps,
                    critic_iterations = :critic, tokens_used = :tokens, cost_kop = :cost,
                    trace = :trace::jsonb, finished_at = now()
                WHERE id = :id
                """, new MapSqlParameterSource()
                .addValue("id", runId)
                .addValue("status", status)
                .addValue("steps", stepsCount)
                .addValue("critic", criticIterations)
                .addValue("tokens", tokensUsed)
                .addValue("cost", costKop)
                .addValue("trace", traceJson == null ? "[]" : traceJson));
    }

    @Override
    public UUID saveInsight(Insight insight, List<EvidenceRow> evidence) {
        try {
            jdbc.update("""
                    INSERT INTO insights (insight_id, story_id, company_id, agent_run_id, model_version,
                        confidence, relevance_score, impact_score, severity, payload)
                    VALUES (:id, :story, :company, :run, :version, :confidence, :relevance, :impact, :severity, :payload::jsonb)
                    ON CONFLICT (story_id, company_id, model_version) DO NOTHING
                    """, new MapSqlParameterSource()
                    .addValue("id", insight.insightId())
                    .addValue("story", insight.story().storyId())
                    .addValue("company", insight.companyId())
                    .addValue("run", insight.agentRunId())
                    .addValue("version", insight.modelVersion())
                    .addValue("confidence", insight.confidence().name())
                    .addValue("relevance", insight.personalization().relevanceScore())
                    .addValue("impact", insight.impactAnalysis().impactScore())
                    .addValue("severity", insight.impactAnalysis().severity())
                    .addValue("payload", mapper.writeValueAsString(insight)));

            Optional<UUID> saved = findInsightId(insight.story().storyId(), insight.companyId(),
                    insight.modelVersion());
            UUID insightId = saved.orElseThrow();
            insertEvidence(insightId, evidence);
            return insightId;
        } catch (Exception e) {
            throw new IllegalStateException("не удалось сохранить инсайт", e);
        }
    }

    private void insertEvidence(UUID insightId, List<EvidenceRow> evidence) {
        for (EvidenceRow row : evidence) {
            jdbc.update("""
                    INSERT INTO evidence (insight_id, field_path, ref_type, ref, attribution_id)
                    VALUES (:insight, :path, :type, :ref,
                        (SELECT id FROM attribution_types WHERE code = :attribution))
                    """, new MapSqlParameterSource()
                    .addValue("insight", insightId)
                    .addValue("path", row.fieldPath())
                    .addValue("type", row.refType())
                    .addValue("ref", row.ref())
                    .addValue("attribution", row.attribution()));
        }
    }
}
