package ru.alfahack.agent.llm;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

import org.springframework.stereotype.Component;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.networknt.schema.JsonSchema;
import com.networknt.schema.JsonSchemaFactory;
import com.networknt.schema.SpecVersion;
import com.networknt.schema.ValidationMessage;

import ru.alfahack.agent.client.GatewayClient;
import ru.alfahack.agent.client.GatewayCompletion;
import ru.alfahack.agent.client.GatewayRequest;
import ru.alfahack.agent.config.AgentProperties;
import lombok.RequiredArgsConstructor;

@Component
@RequiredArgsConstructor
public class StructuredCallService {

    private static final String SERVICE_NAME = "java-agent";
    private static final int MAX_ATTEMPTS = 2;

    private final GatewayClient gateway;
    private final SchemaLoader schemaLoader;
    private final AgentProperties props;
    private final ObjectMapper mapper;
    private final JsonSchemaFactory schemaFactory = JsonSchemaFactory
            .getInstance(SpecVersion.VersionFlag.V202012);
    private final ConcurrentHashMap<String, JsonSchema> schemaCache = new ConcurrentHashMap<>();

    public <T> Outcome<T> call(Task task, String schemaName, Class<T> type,
            List<GatewayRequest.Message> messages, Double temperature, Integer maxTokens,
            SemanticValidator<T> semantic) {
        JsonNode schemaNode = schemaLoader.load(schemaName);
        Map<String, Object> responseFormat = props.gateway().structuredOutputs()
                ? Map.of("type", "json_schema",
                        "json_schema", Map.of("name", schemaName, "schema", schemaNode, "strict", true))
                : null;

        List<GatewayRequest.Message> prompt = messages;
        List<Outcome.Attempt> attempts = new ArrayList<>(MAX_ATTEMPTS);
        List<String> previousErrors = List.of();

        for (int attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
            GatewayCompletion completion = gateway.complete(new GatewayRequest(
                    SERVICE_NAME, props.actor(), task.code(), prompt,
                    temperature, maxTokens, responseFormat, null));
            attempts.add(new Outcome.Attempt(completion.model(), completion.usage(),
                    completion.costKop(), completion.cacheHit()));

            Validated<T> result = validate(completion.content(), schemaName, type, semantic);
            if (result.errors().isEmpty()) {
                return new Outcome<>(result.value(), attempts, attempt > 1);
            }
            previousErrors = result.errors();
            prompt = repairPrompt(messages, completion.content(), previousErrors);
        }

        throw new StructuredOutputException(schemaName, previousErrors);
    }

    private <T> Validated<T> validate(String content, String schemaName, Class<T> type,
            SemanticValidator<T> semantic) {
        List<String> errors = new ArrayList<>();

        String json = extractJson(content);
        if (json == null) {
            errors.add("ответ не содержит JSON-объект");
            return new Validated<>(null, errors);
        }

        JsonNode node;
        try {
            node = mapper.readTree(json);
        } catch (JsonProcessingException e) {
            errors.add("JSON не парсится: " + e.getOriginalMessage());
            return new Validated<>(null, errors);
        }

        JsonSchema schema = schemaCache.computeIfAbsent(schemaName,
                name -> schemaFactory.getSchema(schemaLoader.load(name)));
        for (ValidationMessage message : schema.validate(node)) {
            errors.add(message.getMessage());
        }
        if (!errors.isEmpty()) {
            return new Validated<>(null, errors);
        }

        T value;
        try {
            value = mapper.treeToValue(node, type);
        } catch (JsonProcessingException e) {
            errors.add("ответ не отображается на ожидаемый тип " + type.getSimpleName() + ": "
                    + e.getOriginalMessage());
            return new Validated<>(null, errors);
        }

        if (semantic != null) {
            errors.addAll(semantic.validate(value));
        }
        return new Validated<>(errors.isEmpty() ? value : null, errors);
    }

    static String extractJson(String content) {
        if (content == null || content.isBlank()) {
            return null;
        }
        int start = content.indexOf('{');
        int end = content.lastIndexOf('}');
        if (start < 0 || end <= start) {
            return null;
        }
        return content.substring(start, end + 1);
    }

    private static List<GatewayRequest.Message> repairPrompt(List<GatewayRequest.Message> original,
            String invalidAnswer, List<String> errors) {
        List<GatewayRequest.Message> prompt = new ArrayList<>(original);
        prompt.add(GatewayRequest.Message.assistant(invalidAnswer));
        prompt.add(GatewayRequest.Message.user(
                "Предыдущий ответ не прошёл валидацию JSON Schema. Ошибки:\n- "
                        + String.join("\n- ", errors)
                        + "\nВерни исправленный JSON строго по схеме. Только JSON, без пояснений и markdown."));
        return List.copyOf(prompt);
    }

    public record Outcome<T>(T value, List<Attempt> attempts, boolean repaired) {

        public long totalTokens() {
            return attempts.stream()
                    .mapToLong(a -> a.usage() != null ? a.usage().totalTokens() : 0)
                    .sum();
        }

        public long totalCostKop() {
            return attempts.stream().mapToLong(Attempt::costKop).sum();
        }

        public record Attempt(String model, GatewayCompletion.Usage usage, long costKop, boolean cacheHit) {
        }
    }

    private record Validated<T>(T value, List<String> errors) {
    }
}
