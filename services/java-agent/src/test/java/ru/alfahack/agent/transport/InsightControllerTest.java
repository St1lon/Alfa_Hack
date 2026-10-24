package ru.alfahack.agent.transport;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;

import ru.alfahack.agent.agent.AgentLoop;
import ru.alfahack.agent.agent.AgentRunRepository;
import ru.alfahack.agent.agent.QaAgent;
import ru.alfahack.agent.client.GatewayException;
import ru.alfahack.agent.config.AgentProperties;
import ru.alfahack.agent.config.JacksonConfig;
import ru.alfahack.agent.domain.Insight;

class InsightControllerTest {

    private static final UUID INSIGHT_ID = UUID.nameUUIDFromBytes("insight".getBytes());

    @Test
    void questionRequiresInsightIdAndText() {
        Controller fx = controller();

        var blank = fx.controller.askQuestion("not-a-uuid", Map.of("question", "вопрос"));
        assertThat(blank.getStatusCode()).isEqualTo(HttpStatus.BAD_REQUEST);

        var noQuestion = fx.controller.askQuestion(INSIGHT_ID.toString(), Map.of());
        assertThat(noQuestion.getStatusCode()).isEqualTo(HttpStatus.BAD_REQUEST);

        var tooLong = fx.controller.askQuestion(INSIGHT_ID.toString(),
                Map.of("question", "в".repeat(1001)));
        assertThat(tooLong.getStatusCode()).isEqualTo(HttpStatus.BAD_REQUEST);
    }

    @Test
    void unknownInsightGives404() {
        Controller fx = controller();
        when(fx.runs.findInsight(any())).thenReturn(Optional.empty());

        var response = fx.controller.askQuestion(INSIGHT_ID.toString(), Map.of("question", "что делать?"));

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.NOT_FOUND);
        assertThat(response.getBody()).containsEntry("code", "INSIGHT_NOT_FOUND");
    }

    @Test
    void questionReturnsAnswerWithCitationsAndUsage() {
        Controller fx = controller();
        when(fx.runs.findInsight(any())).thenReturn(Optional.of(insight()));
        when(fx.qa.answer(any(), any())).thenReturn(new QaAgent.Answer(
                "Ставка сохранена — процентные расходы не снизятся.",
                List.of(Map.of("token", "chunk:1", "ref_type", "research_chunk", "ref", "1")),
                List.of("get_company_profile"),
                150, 12));

        var response = fx.controller.askQuestion(INSIGHT_ID.toString(),
                Map.of("question", "что с процентными расходами?"));

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
        assertThat(response.getBody().get("answer")).isEqualTo("Ставка сохранена — процентные расходы не снизятся.");
        assertThat((List<?>) response.getBody().get("citations")).hasSize(1);
        assertThat(response.getBody().get("usage"))
                .isEqualTo(Map.of("tokens", 150L, "cost_kop", 12L));
    }

    @Test
    void budgetExceededGives503() {
        Controller fx = controller();
        when(fx.runs.findInsight(any())).thenReturn(Optional.of(insight()));
        when(fx.qa.answer(any(), any()))
                .thenThrow(new GatewayException.BudgetExceeded("дневной бюджет исчерпан"));

        var response = fx.controller.askQuestion(INSIGHT_ID.toString(), Map.of("question", "вопрос?"));

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.SERVICE_UNAVAILABLE);
        assertThat(response.getBody()).containsEntry("code", "BUDGET_EXCEEDED");
    }

    @Test
    void cardReturnsInsightWithEvidence() {
        Controller fx = controller();
        when(fx.runs.loadCard(any())).thenReturn(Optional.of(new AgentRunRepository.InsightCard(
                insight(),
                List.of(new AgentRunRepository.EvidenceView("impact_analysis.first_order_effect",
                        "news", "1", "FACT")))));

        var response = fx.controller.getCard(INSIGHT_ID.toString());

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
        assertThat(response.getBody()).containsKey("insight");
        assertThat((List<?>) response.getBody().get("evidence")).hasSize(1);
    }

    @Test
    void cardMissingGives404() {
        Controller fx = controller();
        when(fx.runs.loadCard(any())).thenReturn(Optional.empty());

        assertThat(fx.controller.getCard(INSIGHT_ID.toString()).getStatusCode())
                .isEqualTo(HttpStatus.NOT_FOUND);
        assertThat(fx.controller.getCard("junk").getStatusCode()).isEqualTo(HttpStatus.BAD_REQUEST);
    }

    @Test
    void traceReturnsRunSteps() {
        Controller fx = controller();
        when(fx.runs.loadTrace(any())).thenReturn(Optional.of(new AgentRunRepository.RunTrace(
                UUID.nameUUIDFromBytes("run".getBytes()), "FINISHED", 8, 1, 1200, 45,
                "[{\"phase\":\"PLAN\"}]")));

        var response = fx.controller.getTrace(INSIGHT_ID.toString());

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
        assertThat(response.getBody()).containsEntry("status", "FINISHED");
        assertThat(response.getBody().get("steps").toString()).contains("PLAN");
    }

    @Test
    void traceBrokenJsonFallsBackToEmptySteps() {
        Controller fx = controller();
        when(fx.runs.loadTrace(any())).thenReturn(Optional.of(new AgentRunRepository.RunTrace(
                UUID.nameUUIDFromBytes("run".getBytes()), "FINISHED", 8, 1, 1200, 45, "не json")));

        var response = fx.controller.getTrace(INSIGHT_ID.toString());

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
        assertThat((List<?>) response.getBody().get("steps")).isEmpty();
    }

    private Controller controller() {
        AgentLoop loop = mock(AgentLoop.class);
        QaAgent qa = mock(QaAgent.class);
        AgentRunRepository runs = mock(AgentRunRepository.class);
        var mapper = JacksonConfig.createMapper();
        var controller = new InsightController(loop, qa, runs, props(), mapper);
        return new Controller(controller, runs, qa);
    }

    private static AgentProperties props() {
        return new AgentProperties(
                new AgentProperties.Gateway("http://gateway.test", "", Duration.ofSeconds(1),
                        Duration.ofSeconds(1), 3, true),
                new AgentProperties.Rag(0.15, 0.5, 60, 3, 20),
                new AgentProperties.Budget(6, 2, 20000),
                new AgentProperties.Qa(600, 1000, 3),
                "v0.1", "java", new AgentProperties.Selfcheck(false));
    }

    private static Insight insight() {
        return new Insight(
                INSIGHT_ID, "U_67", "v0.1", null, null, Insight.Confidence.HIGH,
                null, null, null, null, null, null);
    }

    private record Controller(InsightController controller, AgentRunRepository runs, QaAgent qa) {
    }
}
