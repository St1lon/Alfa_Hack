package ru.alfahack.agent.client;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

@JsonIgnoreProperties(ignoreUnknown = true)
public record GatewayCompletion(
        String model,
        String content,
        Usage usage,
        long costKop,
        boolean cacheHit,
        boolean softLimitHit,
        long spentTodayKop) {

    @JsonIgnoreProperties(ignoreUnknown = true)
    public record Usage(int promptTokens, int completionTokens, int totalTokens) {
    }
}
