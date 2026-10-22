package ru.alfahack.agent.agent;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

import ru.alfahack.agent.domain.Insight;

public interface AgentRunRepository {

    Optional<UUID> findInsightId(UUID storyId, String companyId, String modelVersion);

    UUID startRun(UUID storyId, String companyId);

    void finishRun(UUID runId, String status, int stepsCount, int criticIterations,
            int tokensUsed, long costKop, String traceJson);

    UUID saveInsight(Insight insight, List<EvidenceRow> evidence);
}
