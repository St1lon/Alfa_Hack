package ru.alfahack.agent.transport;

import java.util.Map;
import java.util.UUID;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

import lombok.RequiredArgsConstructor;

import ru.alfahack.agent.agent.AgentLoop;
import ru.alfahack.agent.client.GatewayException;

@RestController
@RequiredArgsConstructor
public class InsightController {

    private final AgentLoop agentLoop;

    @PostMapping("/v1/insights")
    public ResponseEntity<Map<String, Object>> createInsight(@RequestBody Map<String, String> body) {
        UUID storyId = parseUuid(body == null ? null : body.get("story_id"));
        String companyId = body == null ? null : body.get("company_id");
        if (storyId == null || companyId == null || companyId.isBlank()) {
            return ResponseEntity.badRequest().body(Map.of(
                    "code", "BAD_REQUEST",
                    "description", "нужны story_id (uuid) и company_id"));
        }

        try {
            AgentLoop.InsightOutcome outcome = agentLoop.run(storyId, companyId.strip());
            HttpStatus status = outcome.created() ? HttpStatus.CREATED : HttpStatus.OK;
            return ResponseEntity.status(status).body(Map.of("insight_id", outcome.insightId().toString()));
        } catch (GatewayException.BudgetExceeded e) {
            return ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE).body(Map.of(
                    "code", "BUDGET_EXCEEDED",
                    "description", String.valueOf(e.getMessage())));
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body(Map.of(
                    "code", "AGENT_FAILED",
                    "description", String.valueOf(e.getMessage())));
        }
    }

    @PostMapping("/v1/insights/{insightId}/questions")
    public ResponseEntity<Map<String, String>> askQuestion(@PathVariable String insightId,
            @RequestBody(required = false) Map<String, Object> body) {
        return ResponseEntity.status(HttpStatus.NOT_IMPLEMENTED).body(Map.of(
                "code", "NOT_IMPLEMENTED",
                "description", "Q&A-агент — фаза 4 (J-08)",
                "owner", "Java"));
    }

    private static UUID parseUuid(String value) {
        if (value == null || value.isBlank()) {
            return null;
        }
        try {
            return UUID.fromString(value.strip());
        } catch (IllegalArgumentException e) {
            return null;
        }
    }
}
