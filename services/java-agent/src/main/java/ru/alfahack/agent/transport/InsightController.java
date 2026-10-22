package ru.alfahack.agent.transport;

import java.util.Map;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class InsightController {

    private static final Map<String, String> NOT_IMPLEMENTED = Map.of(
            "code", "NOT_IMPLEMENTED",
            "description", "эндпоинт зарезервирован контрактом, реализация в работе",
            "owner", "Java");

    @PostMapping("/v1/insights")
    public ResponseEntity<Map<String, String>> createInsight(
            @RequestBody(required = false) Map<String, Object> body) {
        return ResponseEntity.status(HttpStatus.NOT_IMPLEMENTED).body(NOT_IMPLEMENTED);
    }

    @PostMapping("/v1/insights/{insightId}/questions")
    public ResponseEntity<Map<String, String>> askQuestion(@PathVariable String insightId,
            @RequestBody(required = false) Map<String, Object> body) {
        return ResponseEntity.status(HttpStatus.NOT_IMPLEMENTED).body(NOT_IMPLEMENTED);
    }
}
