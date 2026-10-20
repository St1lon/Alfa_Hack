package ru.alfahack.agent;

import java.util.Map;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

/**
 * Контур java-agent. Пути — заявленный контракт (TEAM_PLAN §7),
 * тела отдаёт Java-разработчик; пока — 501 NOT_IMPLEMENTED.
 */
@RestController
public class AgentController {

    private static final Map<String, String> NOT_IMPLEMENTED = Map.of(
            "code", "NOT_IMPLEMENTED",
            "description", "эндпоинт зарезервирован контрактом, реализация в работе",
            "owner", "Java");

    @GetMapping("/healthz")
    public Map<String, String> healthz() {
        return Map.of("status", "ok");
    }

    @GetMapping("/readyz")
    public Map<String, String> readyz() {
        return Map.of("status", "ok");
    }

    /** Построить разбор по паре «сюжет — компания»: PLAN → ACT → DRAFT → CRITIC → ADAPT → EMIT. */
    @PostMapping("/v1/insights")
    public ResponseEntity<Map<String, String>> createInsight(@RequestBody(required = false) Map<String, Object> body) {
        return ResponseEntity.status(HttpStatus.NOT_IMPLEMENTED).body(NOT_IMPLEMENTED);
    }

    /** Уточняющий вопрос по готовому инсайту (критерий 10). */
    @PostMapping("/v1/insights/{insightId}/questions")
    public ResponseEntity<Map<String, String>> askQuestion(@PathVariable String insightId,
            @RequestBody(required = false) Map<String, Object> body) {
        return ResponseEntity.status(HttpStatus.NOT_IMPLEMENTED).body(NOT_IMPLEMENTED);
    }
}
