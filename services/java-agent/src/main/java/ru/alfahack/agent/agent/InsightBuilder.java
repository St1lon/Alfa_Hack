package ru.alfahack.agent.agent;

import java.time.OffsetDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;

import org.springframework.stereotype.Component;

import lombok.RequiredArgsConstructor;

import ru.alfahack.agent.cib.CibMatcher;
import ru.alfahack.agent.cib.RiskTag;
import ru.alfahack.agent.domain.Insight;

@Component
@RequiredArgsConstructor
public class InsightBuilder {

    private final CibMatcher cibMatcher;

    public BuildResult build(AgentRunContext ctx, MarketDraft market, CompanyDraft draft,
            Insight.Confidence confidence, TraceStats stats, String modelVersion, UUID runId) {
        UUID insightId = UUID.randomUUID();
        var story = ctx.story();
        var company = ctx.company();

        List<EvidenceRow> evidence = new ArrayList<>();

        Insight.EvidenceRef primaryNews = story.sources().stream()
                .filter(AgentRunContext.StoryBundle.Source::primary)
                .map(s -> new Insight.EvidenceRef("news", s.newsId().toString()))
                .findFirst()
                .orElseGet(() -> story.sources().isEmpty() ? null
                        : new Insight.EvidenceRef("news", story.sources().get(0).newsId().toString()));

        Insight.Effect firstOrder = new Insight.Effect(
                market.firstOrder().text(),
                Insight.Attribution.FACT,
                primaryNews == null ? List.of() : List.of(primaryNews));
        if (primaryNews != null) {
            evidence.add(new EvidenceRow("impact_analysis.first_order_effect", "news", primaryNews.ref(),
                    EvidenceRow.FACT));
        }

        List<Insight.EvidenceRef> secondOrderRefs = draft.secondOrder().citations().stream()
                .map(ctx::resolveCitation)
                .filter(ref -> ref != null)
                .toList();
        secondOrderRefs.forEach(ref -> evidence.add(new EvidenceRow(
                "impact_analysis.second_order_company_effect", ref.type(), ref.ref(), EvidenceRow.AI_INFERENCE)));
        Insight.Effect secondOrder = new Insight.Effect(
                draft.secondOrder().text(), Insight.Attribution.AI_INFERENCE, secondOrderRefs);

        Set<RiskTag> tags = new java.util.HashSet<>();
        draft.riskTags().forEach(code -> RiskTag.parseLenient(code).ifPresent(tags::add));
        var cibMatch = cibMatcher.match(tags, company.marks());

        List<Insight.PlaybookAction> actions = new ArrayList<>();
        for (var action : draft.playbook().whatToDo()) {
            List<Insight.EvidenceRef> refs = action.citations().stream()
                    .map(ctx::resolveCitation)
                    .filter(ref -> ref != null)
                    .toList();
            int index = actions.size();
            refs.forEach(ref -> evidence.add(new EvidenceRow(
                    "actionable_playbook.what_to_do[" + index + "]", ref.type(), ref.ref(),
                    EvidenceRow.AI_INFERENCE)));
            actions.add(new Insight.PlaybookAction(
                    action.text(), Insight.Attribution.AI_INFERENCE, refs));
        }

        var bankData = ctx.bankViewData() == null
                ? new AgentRunContext.BankViewData(false, List.of(), null)
                : ctx.bankViewData();
        bankData.statements().forEach(s -> evidence.add(new EvidenceRow(
                "bank_view.statements", "research_chunk", s.source().chunkId().toString(),
                EvidenceRow.BANK_VIEW)));

        List<String> featuresUsed = ctx.usedCompanyFeatures();
        featuresUsed.forEach(feature -> evidence.add(new EvidenceRow(
                "personalization.company_features_used", "company_feature", feature,
                EvidenceRow.FACT)));

        Insight insight = new Insight(
                insightId,
                company.companyId(),
                modelVersion,
                runId,
                OffsetDateTime.now(),
                confidence,
                new Insight.StoryBlock(
                        story.storyId(),
                        story.title(),
                        story.sources().size(),
                        story.sources().stream()
                                .map(s -> new Insight.Source(s.newsId(), s.publisher(), s.url(),
                                        s.publishedAt(), s.primary()))
                                .toList(),
                        market.firstOrder().factsConfirmed(),
                        market.firstOrder().factsAssumed()),
                new Insight.Personalization(
                        ctx.relevanceScore(),
                        ctx.matchReasons(),
                        featuresUsed),
                new Insight.BankView(bankData.found(), bankData.statements()),
                new Insight.ImpactBlock(
                        draft.impact().type(),
                        draft.impact().severity(),
                        draft.impact().impactScore(),
                        draft.impact().probability(),
                        draft.impact().timeHorizon(),
                        draft.impact().targetPersonas(),
                        firstOrder,
                        secondOrder),
                new Insight.ActionablePlaybook(
                        actions,
                        draft.playbook().whatToWatch(),
                        draft.playbook().risksToConsider(),
                        draft.playbook().questionsForTeam(),
                        cibMatch.orElse(null)),
                new Insight.TraceBlock(
                        stats.stepsCount(),
                        stats.toolsCalled(),
                        stats.criticIterations(),
                        stats.tokensUsed(),
                        stats.costKop() / 100.0));
        return new BuildResult(insight, List.copyOf(evidence));
    }

    public record BuildResult(Insight insight, List<EvidenceRow> evidence) {
    }

    public record TraceStats(int stepsCount, List<String> toolsCalled, int criticIterations,
            long tokensUsed, long costKop) {

        public TraceStats {
            toolsCalled = toolsCalled == null ? List.of() : List.copyOf(toolsCalled);
        }

        public static TraceStats empty() {
            return new TraceStats(0, List.of(), 0, 0, 0);
        }
    }
}
