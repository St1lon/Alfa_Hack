package ru.alfahack.agent.config;

import java.time.Duration;

import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.boot.context.properties.bind.DefaultValue;

@ConfigurationProperties(prefix = "agent")
public record AgentProperties(
        Gateway gateway,
        @DefaultValue Rag rag,
        @DefaultValue("v0.1") String modelVersion,
        @DefaultValue("java") String actor,
        @DefaultValue Selfcheck selfcheck) {

    public record Gateway(
            @DefaultValue("http://llm-gateway:8091") String baseUrl,
            @DefaultValue("") String token,
            @DefaultValue("5s") Duration connectTimeout,
            @DefaultValue("90s") Duration readTimeout,
            @DefaultValue("3") int retries,
            @DefaultValue("true") boolean structuredOutputs) {
    }

    public record Rag(
            @DefaultValue("0.15") double lexicalMinScore,
            @DefaultValue("0.5") double vectorMinScore,
            @DefaultValue("60") int rrfK,
            @DefaultValue("3") int maxStatements,
            @DefaultValue("20") int candidateLimit) {
    }

    public record Selfcheck(@DefaultValue("false") boolean enabled) {
    }
}
