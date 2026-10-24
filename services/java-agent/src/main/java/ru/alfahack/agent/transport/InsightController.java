package ru.alfahack.agent.transport;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

import com.fasterxml.jackson.databind.ObjectMapper;

import lombok.RequiredArgsConstructor;

import ru.alfahack.agent.agent.AgentRunRepository;
import ru.alfahack.agent.agent.AgentLoop;
import ru.alfahack.agent.agent.QaAgent;
import ru.alfahack.agent.client.GatewayException;
import ru.alfahack.agent.config.AgentProperties;

@RestController
@RequiredArgsConstructor
public class InsightController {

    private final AgentLoop agentLoop;
    private final QaAgent qaAgent;
    private final AgentRunRepository runs;
    private final AgentProperties props;
    private final ObjectMapper mapper;

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
    public ResponseEntity<Map<String, Object>> askQuestion(@PathVariable String insightId,
            @RequestBody(required = false) Map<String, Object> body) {
        UUID id = parseUuid(insightId);
        String question = body == null || body.get("question") == null
                ? null
                : String.valueOf(body.get("question")).strip();
        if (id == null || question == null || question.isEmpty()) {
            return ResponseEntity.badRequest().body(Map.of(
                    "code", "BAD_REQUEST",
                    "description", "нужны insight_id (uuid) в пути и непустой question"));
        }
        if (question.length() > props.qa().maxQuestionChars()) {
            return ResponseEntity.badRequest().body(Map.of(
                    "code", "BAD_REQUEST",
                    "description", "вопрос длиннее " + props.qa().maxQuestionChars() + " символов"));
        }

        Optional<ru.alfahack.agent.domain.Insight> insight = runs.findInsight(id);
        if (insight.isEmpty()) {
            return ResponseEntity.status(HttpStatus.NOT_FOUND).body(Map.of(
                    "code", "INSIGHT_NOT_FOUND",
                    "description", "инсайт не построен или id неверный"));
        }

        try {
            QaAgent.Answer answer = qaAgent.answer(insight.get(), question);
            Map<String, Object> response = new LinkedHashMap<>();
            response.put("answer", answer.answer());
            response.put("citations", answer.citations());
            response.put("tools_called", answer.toolsCalled());
            response.put("usage", Map.of("tokens", answer.tokens(), "cost_kop", answer.costKop()));
            return ResponseEntity.ok(response);
        } catch (GatewayException.BudgetExceeded e) {
            return ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE).body(Map.of(
                    "code", "BUDGET_EXCEEDED",
                    "description", String.valueOf(e.getMessage())));
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body(Map.of(
                    "code", "QA_FAILED",
                    "description", String.valueOf(e.getMessage())));
        }
    }

    @GetMapping("/v1/insights/{insightId}")
    public ResponseEntity<Map<String, Object>> getCard(@PathVariable String insightId) {
        UUID id = parseUuid(insightId);
        if (id == null) {
            return badUuid();
        }
        Optional<AgentRunRepository.InsightCard> card = runs.loadCard(id);
        if (card.isEmpty()) {
            return notFound();
        }
        return ResponseEntity.ok(Map.of(
                "insight", mapper.valueToTree(card.get().insight()),
                "evidence", card.get().evidence()));
    }

    @GetMapping("/v1/insights/{insightId}/trace")
    public ResponseEntity<Map<String, Object>> getTrace(@PathVariable String insightId) {
        UUID id = parseUuid(insightId);
        if (id == null) {
            return badUuid();
        }
        Optional<AgentRunRepository.RunTrace> trace = runs.loadTrace(id);
        if (trace.isEmpty()) {
            return notFound();
        }
        var run = trace.get();
        Map<String, Object> response = new LinkedHashMap<>();
        response.put("run_id", run.runId().toString());
        response.put("status", run.status());
        response.put("steps_count", run.stepsCount());
        response.put("critic_iterations", run.criticIterations());
        response.put("tokens_used", run.tokensUsed());
        response.put("cost_kop", run.costKop());
        response.put("steps", readSteps(run.traceJson()));
        return ResponseEntity.ok(response);
    }

    private Object readSteps(String traceJson) {
        try {
            return mapper.readTree(traceJson);
        } catch (Exception e) {
            return List.of();
        }
    }

    private static ResponseEntity<Map<String, Object>> badUuid() {
        return ResponseEntity.badRequest().body(Map.of(
                "code", "BAD_REQUEST",
                "description", "insight_id должен быть uuid"));
    }

    private static ResponseEntity<Map<String, Object>> notFound() {
        return ResponseEntity.status(HttpStatus.NOT_FOUND).body(Map.of(
                "code", "INSIGHT_NOT_FOUND",
                "description", "инсайт не построен или id неверный"));
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
