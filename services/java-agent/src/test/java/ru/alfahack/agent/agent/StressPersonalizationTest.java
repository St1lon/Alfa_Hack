package ru.alfahack.agent.agent;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.time.Duration;
import java.time.OffsetDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicReference;

import org.junit.jupiter.api.Test;

import ru.alfahack.agent.cib.CibCatalog;
import ru.alfahack.agent.cib.CibMatcher;
import ru.alfahack.agent.cib.ClientMarks;
import ru.alfahack.agent.client.GatewayCompletion;
import ru.alfahack.agent.client.GatewayRequest;
import ru.alfahack.agent.config.AgentProperties;
import ru.alfahack.agent.config.JacksonConfig;
import ru.alfahack.agent.domain.Insight;
import ru.alfahack.agent.llm.StructuredCallService;
import ru.alfahack.agent.rag.CompanySnapshot;
import ru.alfahack.agent.rag.QueryComposer;
import ru.alfahack.agent.rag.RagRequest;

class StressPersonalizationTest {

    private static final UUID STORY_ID = UUID.nameUUIDFromBytes("stress-story".getBytes());
    private static final UUID NEWS_ID = UUID.nameUUIDFromBytes("stress-news".getBytes());

    private static final String STORY_TITLE = "Китай ввёл повышенные пошлины на импорт российской металлопродукции";

    @Test
    void sameStoryProducesOppositeInsightsForDifferentClients() {
        Fixture fx = fixture();

        Insight exporter = fx.savedInsights.get(0);
        Insight domestic = fx.savedInsights.get(1);

        assertThat(exporter.impactAnalysis().impactScore()).isEqualTo(-9);
        assertThat(exporter.impactAnalysis().type()).isEqualTo("RISK");
        assertThat(domestic.impactAnalysis().impactScore()).isEqualTo(6);
        assertThat(domestic.impactAnalysis().type()).isEqualTo("OPPORTUNITY");

        var exporterProduct = exporter.actionablePlaybook().alfaCibProductMatch();
        var domesticProduct = domestic.actionablePlaybook().alfaCibProductMatch();
        assertThat(exporterProduct).isNotNull();
        assertThat(domesticProduct).isNotNull();
        assertThat(exporterProduct.productName()).isNotEqualTo(domesticProduct.productName());
        assertThat(exporterProduct.riskTags()).contains("export");
        assertThat(domesticProduct.riskTags()).contains("rate_risk");

        assertThat(exporter.personalization().companyFeaturesUsed()).contains("FTS_COUNTRY=КИТАЙ");
        assertThat(domestic.personalization().companyFeaturesUsed()).contains("CL_PRODUCT_LOA_MARK=true");

        assertThat(fx.exporterPrompt.get()).isNotEqualTo(fx.domesticPrompt.get());
        assertThat(fx.exporterPrompt.get()).contains("КИТАЙ");
        assertThat(fx.domesticPrompt.get()).doesNotContain("FTS_COUNTRY=КИТАЙ");
    }

    @Test
    void ragQueriesDifferForSameStoryAndDifferentClients() {
        var composer = new QueryComposer();
        String exporterQuery = composer.compose(ragRequest(new CompanySnapshot(
                "Металлургия", List.of("КИТАЙ"), false, true, false)));
        String domesticQuery = composer.compose(ragRequest(new CompanySnapshot(
                "Производство стройматериалов", List.of(), true, false, false)));

        assertThat(exporterQuery).isNotEqualTo(domesticQuery);
        assertThat(exporterQuery).contains("КИТАЙ");
        assertThat(domesticQuery).contains("ключевая ставка");
    }

    private static RagRequest ragRequest(CompanySnapshot snapshot) {
        return new RagRequest(STORY_TITLE, "Пошлины подняты на 12%", List.of(),
                java.util.Set.of(ru.alfahack.agent.cib.RiskTag.EXPORT), snapshot);
    }

    private static String schemaOf(org.mockito.invocation.InvocationOnMock inv) {
        return inv.getArgument(1);
    }

    private static PlanOutput plan() {
        return new PlanOutput(List.of("get_company_profile", "list_cib_products"), "профиль и продукты");
    }

    private static StructuredCallService.Outcome<?> outcome(Object value) {
        return new StructuredCallService.Outcome<>(value,
                List.of(new StructuredCallService.Outcome.Attempt("glm-5.3-flash", usage(), 15, false)),
                false);
    }

    private Fixture fixture() {
        StructuredCallService llm = mock(StructuredCallService.class);
        AgentRunRepository runs = mock(AgentRunRepository.class);
        when(runs.findInsightId(any(), anyString(), anyString())).thenReturn(Optional.empty());
        when(runs.startRun(any(), anyString())).thenReturn(UUID.nameUUIDFromBytes("run".getBytes()));
        List<Insight> saved = new ArrayList<>();
        when(runs.saveInsight(any(), anyList())).thenAnswer(inv -> {
            Insight insight = inv.getArgument(0);
            saved.add(insight);
            return insight.insightId();
        });

        AtomicReference<String> exporterPrompt = new AtomicReference<>();
        AtomicReference<String> domesticPrompt = new AtomicReference<>();
        when(llm.call(any(), anyString(), any(), anyList(), any(), any(), any())).thenAnswer(inv -> {
            List<GatewayRequest.Message> messages = inv.getArgument(3);
            String prompt = messages.get(1).content();
            boolean exporter = prompt.contains("КИТАЙ");
            (exporter ? exporterPrompt : domesticPrompt).set(prompt);
            return outcome(switch (schemaOf(inv)) {
                case "plan" -> plan();
                case "draft_market" -> tradeMarketDraft();
                case "draft_company" -> exporter ? exporterDraft() : domesticDraft();
                default -> criticPassed();
            });
        });

        var mapper = JacksonConfig.createMapper();
        var registry = new ToolRegistry(List.of(
                new ru.alfahack.agent.agent.tools.GetCompanyProfileTool(),
                new ru.alfahack.agent.agent.tools.ListCibProductsTool(new CibCatalog(mapper))));
        var builder = new InsightBuilder(new CibMatcher(new CibCatalog(mapper)));
        List<AgentRunContext> contexts = new ArrayList<>();
        contexts.add(context("U_80", exporterCompany()));
        contexts.add(context("U_90", domesticCompany()));
        ContextLoader loader = (storyId, companyId) -> "U_80".equals(companyId) ? contexts.get(0) : contexts.get(1);

        var loop = new AgentLoop(llm, loader, registry, builder, runs, props(), mapper);
        loop.run(STORY_ID, "U_80");
        loop.run(STORY_ID, "U_90");
        return new Fixture(llm, loop, saved, exporterPrompt, domesticPrompt);
    }

    private static AgentRunContext context(String companyId, AgentRunContext.CompanyBundle company) {
        var story = new AgentRunContext.StoryBundle(
                STORY_ID, STORY_TITLE,
                "Пошлины подняты на 12% для сортового проката",
                List.of(new AgentRunContext.StoryBundle.Source(NEWS_ID, "Интерфакс", "https://example.com",
                        OffsetDateTime.parse("2026-10-24T08:00:00Z"), true)),
                List.of(ru.alfahack.agent.cib.RiskTag.EXPORT,
                        ru.alfahack.agent.cib.RiskTag.SUPPLY_CHAIN),
                List.of("КИТАЙ"));
        return new AgentRunContext(STORY_ID, companyId, story, company, List.of(), List.of(), List.of(),
                0.91, List.of(new Insight.MatchReason("MAINOKVED_CODE=" + company.okvedCode(),
                        company.okvedName(), 0.31)));
    }

    private static AgentRunContext.CompanyBundle exporterCompany() {
        return new AgentRunContext.CompanyBundle(
                "U_80", "Экспортёр металлопроката", "ООО", null, null, null,
                "24.10", "Металлургия", "1000+", "действующий", 5.0e7, null,
                "Челябинск", "Челябинская область", true, List.of("КИТАЙ"),
                new ClientMarks(false, true, false, false, false, false, false),
                List.of(
                        new AgentRunContext.CompanyBundle.Feature("FTS_COUNTRY", "КИТАЙ",
                                "fts_countries", "КИТАЙ"),
                        new AgentRunContext.CompanyBundle.Feature("CL_PRODUCT_GAR_MARK", "true",
                                "companies.cl_product_gar_mark", "true")));
    }

    private static AgentRunContext.CompanyBundle domesticCompany() {
        return new AgentRunContext.CompanyBundle(
                "U_90", "Завод стройматериалов", "АО", null, null, null,
                "23.99", "Производство стройматериалов", "500+", "действующий", 8.0e6, null,
                "Тула", "Тульская область", false, List.of(),
                new ClientMarks(false, false, true, false, false, false, false),
                List.of(new AgentRunContext.CompanyBundle.Feature("CL_PRODUCT_LOA_MARK", "true",
                        "companies.cl_product_loa_mark", "true")));
    }

    private static MarketDraft tradeMarketDraft() {
        return new MarketDraft(new MarketDraft.FirstOrder(
                "Экспорт металлопродукции в Китай дорожает, рентабельность поставок падает.",
                List.of(new MarketDraft.Driver("Пошлины +12% на сортовой прокат", false))));
    }

    private static CompanyDraft exporterDraft() {
        return new CompanyDraft(
                new CompanyDraft.SecondOrder(
                        "Компания отгружает в Китай значительную часть выручки: пошлины съедают маржу "
                                + "и повышают риск кассового разрыва в следующем квартале.",
                        List.of("feat:MAINOKVED_CODE", "feat:FTS_COUNTRY")),
                new CompanyDraft.Impact("RISK", "HIGH", -9, 0.8, "SHORT_TERM_1_3M", List.of("CFO", "CEO")),
                List.of("export", "trade_restriction", "supply_chain"),
                new CompanyDraft.Playbook(
                        List.of(new CompanyDraft.Action(
                                "Оценить переход на расчёты с покрытием банковскими гарантиями",
                                List.of("feat:CL_PRODUCT_GAR_MARK"))),
                        List.of("Ответные меры правительства"),
                        List.of("Срыв годового контракта с китайским покупателем"),
                        List.of("Какая доля выручки приходится на Китай?")));
    }

    private static CompanyDraft domesticDraft() {
        return new CompanyDraft(
                new CompanyDraft.SecondOrder(
                        "Импортный прокат дорожает — конкуренты-импортёры поднимают цены, спрос "
                                + "перетекает на локальных производителей, загрузка мощностей растёт.",
                        List.of("feat:MAINOKVED_CODE", "feat:CL_PRODUCT_LOA_MARK")),
                new CompanyDraft.Impact("OPPORTUNITY", "MEDIUM", 6, 0.6, "MID_TERM_3_12M", List.of("CEO")),
                List.of("rate_risk"),
                new CompanyDraft.Playbook(
                        List.of(new CompanyDraft.Action(
                                "Проверить лимит кредитной линии под закупку сырья вперёд",
                                List.of("feat:CL_PRODUCT_LOA_MARK"))),
                        List.of("Индексы цен на металлопродукцию"),
                        List.of("Откат пошлин вернёт демпинг импортёров"),
                        List.of("Есть ли мощность под рост загрузки?")));
    }

    private static CriticReport criticPassed() {
        return new CriticReport(true, List.of());
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

    private static GatewayCompletion.Usage usage() {
        return new GatewayCompletion.Usage(100, 50, 150);
    }

    private record Fixture(StructuredCallService llm, AgentLoop loop, List<Insight> savedInsights,
            AtomicReference<String> exporterPrompt, AtomicReference<String> domesticPrompt) {
    }
}
