package ru.alfahack.agent.llm;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.time.Duration;
import java.util.List;

import org.junit.jupiter.api.Test;

import ru.alfahack.agent.client.GatewayClient;
import ru.alfahack.agent.client.GatewayCompletion;
import ru.alfahack.agent.client.GatewayRequest;
import ru.alfahack.agent.config.AgentProperties;
import ru.alfahack.agent.domain.ExtractedFacts;

class StructuredCallServiceTest {

    private static final String VALID_FACTS = """
            {"facts_confirmed": ["Ключевая ставка сохранена на уровне 14%"],
             "facts_assumed": []}""";

    @Test
    void returnsValueOnFirstAttempt() {
        GatewayClient gateway = mock(GatewayClient.class);
        when(gateway.complete(any())).thenReturn(completion(VALID_FACTS));
        StructuredCallService service = service(gateway, true);

        StructuredCallService.Outcome<ExtractedFacts> outcome = call(service);

        assertThat(outcome.repaired()).isFalse();
        assertThat(outcome.attempts()).hasSize(1);
        assertThat(outcome.value().factsConfirmed()).containsExactly("Ключевая ставка сохранена на уровне 14%");
    }

    @Test
    void repairsOnceWhenSchemaFails() {
        String broken = "{\"facts_confirmed\": \"ставка сохранена\", \"facts_assumed\": []}";
        GatewayClient gateway = mock(GatewayClient.class);
        when(gateway.complete(any())).thenReturn(completion(broken), completion(VALID_FACTS));
        StructuredCallService service = service(gateway, true);

        StructuredCallService.Outcome<ExtractedFacts> outcome = call(service);

        assertThat(outcome.repaired()).isTrue();
        assertThat(outcome.attempts()).hasSize(2);
        assertThat(outcome.value().factsConfirmed()).isNotEmpty();

        var repair = org.mockito.ArgumentCaptor.forClass(GatewayRequest.class);
        verify(gateway, times(2)).complete(repair.capture());
        var secondMessages = repair.getAllValues().get(1).messages();
        assertThat(secondMessages).hasSize(3);
        assertThat(secondMessages.get(1).role()).isEqualTo("assistant");
        assertThat(secondMessages.get(2).content()).contains("JSON Schema");
    }

    @Test
    void throwsAfterFailedRepair() {
        String broken = "никакого JSON здесь нет";
        GatewayClient gateway = mock(GatewayClient.class);
        when(gateway.complete(any())).thenReturn(completion(broken));
        StructuredCallService service = service(gateway, true);

        assertThatThrownBy(() -> call(service))
                .isInstanceOf(StructuredOutputException.class)
                .hasMessageContaining("extract_facts");
        verify(gateway, times(2)).complete(any());
    }

    @Test
    void stripsMarkdownFencesAroundJson() {
        String fenced = "```json\n" + VALID_FACTS + "\n```";
        GatewayClient gateway = mock(GatewayClient.class);
        when(gateway.complete(any())).thenReturn(completion(fenced));
        StructuredCallService service = service(gateway, true);

        assertThat(call(service).value().factsConfirmed()).isNotEmpty();
    }

    @Test
    void omitsResponseFormatWhenStructuredOutputsDisabled() {
        GatewayClient gateway = mock(GatewayClient.class);
        when(gateway.complete(any())).thenReturn(completion(VALID_FACTS));
        StructuredCallService service = service(gateway, false);

        call(service);

        var captor = org.mockito.ArgumentCaptor.forClass(GatewayRequest.class);
        verify(gateway).complete(captor.capture());
        assertThat(captor.getValue().responseFormat()).isNull();
        assertThat(captor.getValue().task()).isEqualTo("extract");
        assertThat(captor.getValue().service()).isEqualTo("java-agent");
    }

    @Test
    void runsSemanticValidatorOnTopOfSchema() {
        String emptyFacts = "{\"facts_confirmed\": [], \"facts_assumed\": []}";
        GatewayClient gateway = mock(GatewayClient.class);
        when(gateway.complete(any())).thenReturn(completion(emptyFacts), completion(VALID_FACTS));
        StructuredCallService service = service(gateway, true);

        var outcome = service.call(
                Task.EXTRACT, "extract_facts", ExtractedFacts.class,
                List.of(GatewayRequest.Message.user("новость")),
                0.1, 600,
                facts -> facts.factsConfirmed().isEmpty() && facts.factsAssumed().isEmpty()
                        ? List.of("оба списка фактов пусты — факты не извлечены")
                        : List.of());

        assertThat(outcome.repaired()).isTrue();
        assertThat(outcome.value().factsConfirmed()).isNotEmpty();
    }

    private static StructuredCallService.Outcome<ExtractedFacts> call(StructuredCallService service) {
        return service.call(
                Task.EXTRACT, "extract_facts", ExtractedFacts.class,
                List.of(GatewayRequest.Message.user("новость")),
                0.1, 600, null);
    }

    private static StructuredCallService service(GatewayClient gateway, boolean structuredOutputs) {
        AgentProperties props = new AgentProperties(
                new AgentProperties.Gateway("http://gateway.test", "", Duration.ofSeconds(1),
                        Duration.ofSeconds(1), 3, structuredOutputs),
                "v0.1", "java", new AgentProperties.Selfcheck(false));
        var mapper = ru.alfahack.agent.config.JacksonConfig.createMapper();
        return new StructuredCallService(gateway, new SchemaLoader(mapper), props, mapper);
    }

    private static GatewayCompletion completion(String content) {
        return new GatewayCompletion("qwen3.6-35b-a3b", content,
                new GatewayCompletion.Usage(10, 5, 15), 7, false, false, 700);
    }
}
