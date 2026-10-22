package ru.alfahack.agent.agent;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.time.Duration;
import java.time.OffsetDateTime;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;

import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import ru.alfahack.agent.cib.CibCatalog;
import ru.alfahack.agent.cib.CibMatcher;
import ru.alfahack.agent.cib.ClientMarks;
import ru.alfahack.agent.client.GatewayCompletion;
import ru.alfahack.agent.config.AgentProperties;
import ru.alfahack.agent.config.JacksonConfig;
import ru.alfahack.agent.domain.Insight;
import ru.alfahack.agent.llm.StructuredCallService;

class AgentLoopTest {

    private static final UUID STORY_ID = UUID.nameUUIDFromBytes("story".getBytes());
    private static final UUID NEWS_ID = UUID.nameUUIDFromBytes("news".getBytes());

    @Test
    void happyPathBuildsInsightWithEvidenceAndProductMatch() {
        Fixture fx = fixture();
        when(fx.llm.call(any(), anyString(), any(), anyList(), any(), any(), any())).thenAnswer(inv -> {
            String schema = inv.getArgument(1);
            return new StructuredCallService.Outcome<>(
                    switch (schema) {
                        case "plan" -> plan();
                        case "draft_market" -> marketDraft();
                        case "draft_company" -> companyDraft();
                        default -> critic(true);
                    },
                    List.of(new StructuredCallService.Outcome.Attempt("glm-5.3-flash", usage(), 15, false)),
                    false);
        });

        AgentLoop.InsightOutcome outcome = fx.loop.run(STORY_ID, "U_67");

        assertThat(outcome.created()).isTrue();
        assertThat(outcome.degraded()).isFalse();

        var insight = captureInsight(fx);
        assertThat(insight.confidence()).isEqualTo(Insight.Confidence.HIGH);
        assertThat(insight.impactAnalysis().impactScore()).isEqualTo(-8);
        assertThat(insight.impactAnalysis().firstOrderEffect().attribution())
                .isEqualTo(Insight.Attribution.FACT);
        assertThat(insight.impactAnalysis().secondOrderCompanyEffect().evidence()).hasSize(2);
        assertThat(insight.actionablePlaybook().alfaCibProductMatch()).isNotNull();
        assertThat(insight.actionablePlaybook().alfaCibProductMatch().clientHasProduct()).isTrue();
        assertThat(insight.personalization().companyFeaturesUsed())
                .contains("MAINOKVED_CODE=24.10", "CL_PRODUCT_LOA_MARK=true");

        var evidence = captureEvidence(fx);
        assertThat(evidence).anySatisfy(row -> {
            assertThat(row.fieldPath()).isEqualTo("impact_analysis.first_order_effect");
            assertThat(row.refType()).isEqualTo("news");
            assertThat(row.attribution()).isEqualTo(EvidenceRow.FACT);
        });
        assertThat(evidence).anySatisfy(row -> {
            assertThat(row.fieldPath()).isEqualTo("impact_analysis.second_order_company_effect");
            assertThat(row.attribution()).isEqualTo(EvidenceRow.AI_INFERENCE);
        });

        ArgumentCaptor<String> status = ArgumentCaptor.forClass(String.class);
        verify(fx.runs).finishRun(any(), status.capture(), anyInt(), anyInt(), anyInt(), anyLong(), anyString());
        assertThat(status.getValue()).isEqualTo("FINISHED");
    }

    @Test
    void criticFailureTriggersAdaptationAndSecondDraft() {
        Fixture fx = fixture();
        AtomicInteger companyDraftCalls = new AtomicInteger();
        when(fx.llm.call(any(), anyString(), any(), anyList(), any(), any(), any())).thenAnswer(inv -> {
            String schema = inv.getArgument(1);
            return new StructuredCallService.Outcome<>(
                    switch (schema) {
                        case "plan" -> plan();
                        case "draft_market" -> marketDraft();
                        case "draft_company" -> {
                            companyDraftCalls.incrementAndGet();
                            yield companyDraft();
                        }
                        default -> critic(companyDraftCalls.get() >= 2);
                    },
                    List.of(new StructuredCallService.Outcome.Attempt("glm-5.3-flash", usage(), 15, false)),
                    false);
        });

        AgentLoop.InsightOutcome outcome = fx.loop.run(STORY_ID, "U_67");

        assertThat(outcome.degraded()).isFalse();
        verify(fx.llm, times(6)).call(any(), anyString(), any(), anyList(), any(), any(), any());

        ArgumentCaptor<String> prompts = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<Integer> critics = ArgumentCaptor.forClass(Integer.class);
        verify(fx.runs).finishRun(any(), anyString(), anyInt(), critics.capture(), anyInt(), anyLong(), anyString());
        assertThat(critics.getValue()).isEqualTo(2);
        assertThat(captureInsight(fx).confidence()).isEqualTo(Insight.Confidence.HIGH);
    }

    @Test
    void twoFailedCriticsDegradeToLowConfidence() {
        Fixture fx = fixture();
        when(fx.llm.call(any(), anyString(), any(), anyList(), any(), any(), any())).thenAnswer(inv -> {
            String schema = inv.getArgument(1);
            return new StructuredCallService.Outcome<>(
                    switch (schema) {
                        case "plan" -> plan();
                        case "draft_market" -> marketDraft();
                        case "draft_company" -> companyDraft();
                        default -> critic(false);
                    },
                    List.of(new StructuredCallService.Outcome.Attempt("glm-5.3-flash", usage(), 15, false)),
                    false);
        });

        AgentLoop.InsightOutcome outcome = fx.loop.run(STORY_ID, "U_67");

        assertThat(outcome.degraded()).isTrue();
        assertThat(captureInsight(fx).confidence()).isEqualTo(Insight.Confidence.LOW);
        ArgumentCaptor<String> status = ArgumentCaptor.forClass(String.class);
        verify(fx.runs).finishRun(any(), status.capture(), anyInt(), anyInt(), anyInt(), anyLong(), anyString());
        assertThat(status.getValue()).isEqualTo("DEGRADED");
    }

    @Test
    void existingInsightIsReturnedWithoutModelCalls() {
        Fixture fx = fixture();
        UUID existing = UUID.nameUUIDFromBytes("existing".getBytes());
        when(fx.runs.findInsightId(any(), anyString(), anyString())).thenReturn(Optional.of(existing));

        AgentLoop.InsightOutcome outcome = fx.loop.run(STORY_ID, "U_67");

        assertThat(outcome.created()).isFalse();
        assertThat(outcome.insightId()).isEqualTo(existing);
        verify(fx.llm, times(0)).call(any(), anyString(), any(), anyList(), any(), any(), any());
    }

    private Insight captureInsight(Fixture fx) {
        ArgumentCaptor<Insight> captor = ArgumentCaptor.forClass(Insight.class);
        verify(fx.runs).saveInsight(captor.capture(), anyList());
        return captor.getValue();
    }

    private List<EvidenceRow> captureEvidence(Fixture fx) {
        ArgumentCaptor<List<EvidenceRow>> captor = ArgumentCaptor.forClass(List.class);
        verify(fx.runs).saveInsight(any(), captor.capture());
        return captor.getValue();
    }

    private Fixture fixture() {
        StructuredCallService llm = mock(StructuredCallService.class);
        AgentRunRepository runs = mock(AgentRunRepository.class);
        when(runs.findInsightId(any(), anyString(), anyString())).thenReturn(Optional.empty());
        when(runs.startRun(any(), anyString())).thenReturn(UUID.nameUUIDFromBytes("run".getBytes()));
        when(runs.saveInsight(any(), anyList())).thenAnswer(inv -> ((Insight) inv.getArgument(0)).insightId());

        var mapper = JacksonConfig.createMapper();
        var registry = new ToolRegistry(List.of(
                new ru.alfahack.agent.agent.tools.GetCompanyProfileTool(),
                new ru.alfahack.agent.agent.tools.ListCibProductsTool(new CibCatalog(mapper))));
        var builder = new InsightBuilder(new CibMatcher(new CibCatalog(mapper)));
        var loop = new AgentLoop(llm, (storyId, companyId) -> context(), registry, builder, runs,
                props(), mapper);
        return new Fixture(llm, runs, loop);
    }

    private static AgentRunContext context() {
        var story = new AgentRunContext.StoryBundle(
                STORY_ID,
                "ЦБ сохранил ключевую ставку на уровне 14%",
                "Первая пауза в цикле смягчения",
                List.of(new AgentRunContext.StoryBundle.Source(NEWS_ID, "Интерфакс", "https://example.com",
                        OffsetDateTime.parse("2026-10-22T08:00:00Z"), true)),
                List.of(), List.of());
        var company = new AgentRunContext.CompanyBundle(
                "U_67", "Синтетический металлург", "ООО", null, null, null,
                "24.10", "Металлургия", "1000+", "действующий", 1.0e7, "G_1",
                "Екатеринбург", "Свердловская область", true, List.of("КИТАЙ"),
                new ClientMarks(true, false, true, false, false, false, false),
                List.of(
                        new AgentRunContext.CompanyBundle.Feature("CL_PRODUCT_LOA_MARK", "true",
                                "companies.cl_product_loa_mark", "true"),
                        new AgentRunContext.CompanyBundle.Feature("FTS_FLAG", "true",
                                "companies.fts_flag", "true")));
        return new AgentRunContext(STORY_ID, "U_67", story, company, List.of(), List.of(), List.of(),
                0.87, List.of(new Insight.MatchReason("MAINOKVED_CODE=24.10",
                        "Металлургия", 0.31)));
    }

    private static AgentProperties props() {
        return new AgentProperties(
                new AgentProperties.Gateway("http://gateway.test", "", Duration.ofSeconds(1),
                        Duration.ofSeconds(1), 3, true),
                new AgentProperties.Rag(0.15, 0.5, 60, 3, 20),
                new AgentProperties.Budget(6, 2, 20000),
                "v0.1", "java", new AgentProperties.Selfcheck(false));
    }

    private static PlanOutput plan() {
        return new PlanOutput(List.of("get_company_profile", "list_cib_products"), "профиль и продукты");
    }

    private static MarketDraft marketDraft() {
        return new MarketDraft(new MarketDraft.FirstOrder(
                "Деньги остаются дорогими для всей экономики: цикл удешевления кредитов приостановлен.",
                List.of(
                        new MarketDraft.Driver("Ставка сохранена на уровне 14%", false),
                        new MarketDraft.Driver("Пауза смягчения может затянуться", true))));
    }

    private static CompanyDraft companyDraft() {
        return new CompanyDraft(
                new CompanyDraft.SecondOrder(
                        "Компания обслуживает кредиты по плавающей ставке — стоимость долга не снизится, "
                                + "а процентные расходы останутся высокой статьёй затрат в ближайший квартал.",
                        List.of("feat:MAINOKVED_CODE", "feat:CL_PRODUCT_LOA_MARK")),
                new CompanyDraft.Impact("RISK", "HIGH", -8, 0.7, "SHORT_TERM_1_3M", List.of("CFO")),
                List.of("rate_risk", "liquidity_gap"),
                new CompanyDraft.Playbook(
                        List.of(new CompanyDraft.Action(
                                "Рассмотреть фиксацию ставки по кредитной линии на 6–12 месяцев",
                                List.of("feat:CL_PRODUCT_LOA_MARK"))),
                        List.of("Календарь заседаний ЦБ"),
                        List.of("Рост процентных расходов при новой tranche"),
                        List.of("Какая доля долга по плавающей ставке?")));
    }

    private static CriticReport critic(boolean passed) {
        return new CriticReport(passed, passed ? List.of()
                : List.of(new CriticReport.Failure("MIN_TWO_COMPANY_FEATURES", "использован один признак")));
    }

    private static GatewayCompletion.Usage usage() {
        return new GatewayCompletion.Usage(100, 50, 150);
    }

    private record Fixture(StructuredCallService llm, AgentRunRepository runs, AgentLoop loop) {
    }
}
