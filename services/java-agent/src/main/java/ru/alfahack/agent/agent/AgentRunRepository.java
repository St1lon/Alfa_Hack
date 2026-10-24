package ru.alfahack.agent.agent;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

import ru.alfahack.agent.domain.Insight;

public interface AgentRunRepository {

    Optional<UUID> findInsightId(UUID storyId, String companyId, String modelVersion);

    Optional<Insight> findInsight(UUID insightId);

    Optional<InsightCard> loadCard(UUID insightId);

    Optional<RunTrace> loadTrace(UUID insightId);

    UUID startRun(UUID storyId, String companyId);

    void finishRun(UUID runId, String status, int stepsCount, int criticIterations,
            int tokensUsed, long costKop, String traceJson);

    UUID saveInsight(Insight insight, List<EvidenceRow> evidence);

    record InsightCard(Insight insight, List<EvidenceView> evidence) {
    }

    record EvidenceView(String fieldPath, String refType, String ref, String attribution) {
    }

    record RunTrace(UUID runId, String status, int stepsCount, int criticIterations,
            int tokensUsed, long costKop, String traceJson) {
    }
}
