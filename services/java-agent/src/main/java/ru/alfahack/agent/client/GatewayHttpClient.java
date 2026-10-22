package ru.alfahack.agent.client;

import java.io.IOException;
import java.net.http.HttpClient;
import java.time.Duration;

import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.http.client.JdkClientHttpRequestFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.client.ResourceAccessException;
import org.springframework.web.client.RestClient;
import org.springframework.web.client.RestClientResponseException;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;

import ru.alfahack.agent.config.AgentProperties;

@Component
public class GatewayHttpClient implements GatewayClient {

    private static final String COMPLETE_PATH = "/v1/complete";

    private final AgentProperties.Gateway props;
    private final ObjectMapper mapper;
    private final RestClient restClient;
    private final Duration backoffBase;

    public GatewayHttpClient(AgentProperties properties, ObjectMapper mapper) {
        this(properties, mapper, defaultRestClient(properties.gateway()), Duration.ofMillis(500));
    }

    GatewayHttpClient(AgentProperties properties, ObjectMapper mapper, RestClient restClient,
            Duration backoffBase) {
        this.props = properties.gateway();
        this.mapper = mapper;
        this.restClient = restClient;
        this.backoffBase = backoffBase;
    }

    private static RestClient defaultRestClient(AgentProperties.Gateway props) {
        HttpClient jdkClient = HttpClient.newBuilder()
                .connectTimeout(props.connectTimeout())
                .build();
        JdkClientHttpRequestFactory factory = new JdkClientHttpRequestFactory(jdkClient);
        factory.setReadTimeout(props.readTimeout());

        RestClient.Builder builder = RestClient.builder()
                .baseUrl(props.baseUrl())
                .requestFactory(factory);
        if (!props.token().isBlank()) {
            builder = builder.defaultHeader(HttpHeaders.AUTHORIZATION, "Bearer " + props.token());
        }
        return builder.build();
    }

    @Override
    public GatewayCompletion complete(GatewayRequest request) {
        String body;
        try {
            body = mapper.writeValueAsString(request);
        } catch (JsonProcessingException e) {
            throw new GatewayException("не удалось сериализовать запрос к gateway", e);
        }

        int attempts = Math.max(1, props.retries());
        RestClientResponseException last = null;
        ResourceAccessException lastIo = null;

        for (int attempt = 1; attempt <= attempts; attempt++) {
            try {
                String response = restClient.post()
                        .uri(COMPLETE_PATH)
                        .contentType(MediaType.APPLICATION_JSON)
                        .body(body)
                        .retrieve()
                        .body(String.class);
                return mapper.readValue(response == null ? "{}" : response, GatewayCompletion.class);
            } catch (JsonProcessingException e) {
                throw new GatewayException("не удалось разобрать ответ gateway", e);
            } catch (RestClientResponseException e) {
                switch (e.getStatusCode().value()) {
                    case 400, 401 -> throw new GatewayException(
                            "gateway отклонил запрос: %s (%s)".formatted(errorDescription(e), e.getStatusCode()),
                            e);
                    case 429 -> throw new GatewayException.BudgetExceeded(
                            "дневной бюджет моделей исчерпан: %s".formatted(errorDescription(e)));
                    default -> {
                        last = e;
                        if (attempt < attempts) {
                            sleep(backoffBase.multipliedBy(1L << (attempt - 1)));
                        }
                    }
                }
            } catch (ResourceAccessException e) {
                lastIo = e;
                if (attempt < attempts) {
                    sleep(backoffBase.multipliedBy(1L << (attempt - 1)));
                }
            }
        }

        if (last != null) {
            throw new GatewayException.UpstreamUnavailable(
                    "gateway недоступен после %d попыток: %s %s".formatted(attempts, last.getStatusCode(),
                            errorDescription(last)), last);
        }
        throw new GatewayException.UpstreamUnavailable(
                "gateway недоступен после %d попыток: %s".formatted(attempts,
                        String.valueOf(lastIo != null ? lastIo.getMessage() : "неизвестная причина")), lastIo);
    }

    private String errorDescription(RestClientResponseException e) {
        try {
            var node = mapper.readTree(e.getResponseBodyAsString());
            if (node.hasNonNull("description")) {
                return node.get("description").asText();
            }
            if (node.hasNonNull("code")) {
                return node.get("code").asText();
            }
        } catch (IOException ignored) {
        }
        return e.getResponseBodyAsString().isBlank() ? "(пустое тело)" : e.getResponseBodyAsString();
    }

    private void sleep(Duration delay) {
        try {
            Thread.sleep(delay.toMillis());
        } catch (InterruptedException ie) {
            Thread.currentThread().interrupt();
            throw new GatewayException("прервано во время бэкоффа перед повтором вызова gateway", ie);
        }
    }
}
