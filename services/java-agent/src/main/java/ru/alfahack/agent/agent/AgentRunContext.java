package ru.alfahack.agent.agent;

import java.time.OffsetDateTime;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;

import ru.alfahack.agent.cib.ClientMarks;
import ru.alfahack.agent.cib.RiskTag;
import ru.alfahack.agent.domain.Insight;

public final class AgentRunContext {

    private final UUID storyId;
    private final String companyId;
    private final StoryBundle story;
    private final CompanyBundle company;
    private final List<Counterparty> counterparties;
    private final List<GroupMember> groupMembers;
    private final List<SimilarStory> similarStories;
    private final Double relevanceScore;
    private final List<Insight.MatchReason> matchReasons;
    private final Map<String, String> toolResults = new LinkedHashMap<>();
    private final Map<String, Insight.EvidenceRef> citations = new LinkedHashMap<>();
    private BankViewData bankViewData;

    public AgentRunContext(UUID storyId, String companyId, StoryBundle story, CompanyBundle company,
            List<Counterparty> counterparties, List<GroupMember> groupMembers,
            List<SimilarStory> similarStories, Double relevanceScore,
            List<Insight.MatchReason> matchReasons) {
        this.storyId = storyId;
        this.companyId = companyId;
        this.story = story;
        this.company = company;
        this.counterparties = List.copyOf(counterparties);
        this.groupMembers = List.copyOf(groupMembers);
        this.similarStories = List.copyOf(similarStories);
        this.relevanceScore = relevanceScore;
        this.matchReasons = List.copyOf(matchReasons);
        registerStoryCitations();
    }

    public UUID storyId() {
        return storyId;
    }

    public String companyId() {
        return companyId;
    }

    public StoryBundle story() {
        return story;
    }

    public CompanyBundle company() {
        return company;
    }

    public List<Counterparty> counterparties() {
        return counterparties;
    }

    public List<GroupMember> groupMembers() {
        return groupMembers;
    }

    public List<SimilarStory> similarStories() {
        return similarStories;
    }

    public Double relevanceScore() {
        return relevanceScore;
    }

    public List<Insight.MatchReason> matchReasons() {
        return matchReasons;
    }

    public Map<String, String> toolResults() {
        return Map.copyOf(toolResults);
    }

    public BankViewData bankViewData() {
        return bankViewData;
    }

    public void addToolResult(String tool, String json) {
        toolResults.put(tool, json);
    }

    public void registerCounterparties(List<Counterparty> top) {
        for (int i = 0; i < top.size(); i++) {
            Counterparty c = top.get(i);
            citations.put("feat:counterparty_" + (i + 1), new Insight.EvidenceRef("company_feature",
                    "counterparty_" + c.direction() + "=" + c.counterpartyId()));
        }
    }

    public void setBankViewData(BankViewData data) {
        this.bankViewData = data;
        data.statements().forEach(s -> citations.put("chunk:" + s.source().chunkId(),
                new Insight.EvidenceRef("research_chunk", s.source().chunkId().toString())));
    }

    public Insight.EvidenceRef resolveCitation(String token) {
        return citations.get(token);
    }

    public List<String> resolveCitations(List<String> tokens) {
        if (tokens == null) {
            return List.of();
        }
        List<String> refs = new ArrayList<>();
        for (String token : tokens) {
            Insight.EvidenceRef ref = citations.get(token);
            if (ref != null) {
                refs.add(ref.ref());
            }
        }
        return refs;
    }

    public List<String> usedCompanyFeatures() {
        return citations.values().stream()
                .filter(ref -> "company_feature".equals(ref.type()))
                .map(Insight.EvidenceRef::ref)
                .distinct()
                .toList();
    }

    public List<String> citationTokens() {
        return List.copyOf(citations.keySet());
    }

    private void registerStoryCitations() {
        for (StoryBundle.Source source : story.sources()) {
            citations.put("news:" + source.newsId(), new Insight.EvidenceRef("news", source.newsId().toString()));
        }
        for (CompanyBundle.Feature feature : company.features()) {
            citations.put("feat:" + feature.key(), new Insight.EvidenceRef("company_feature",
                    feature.key() + "=" + feature.shortValue()));
        }
        citations.put("feat:MAINOKVED_CODE", new Insight.EvidenceRef("company_feature",
                "MAINOKVED_CODE=" + company.okvedCode()));
    }

    public record StoryBundle(
            UUID storyId,
            String title,
            String summary,
            List<Source> sources,
            List<RiskTag> riskTags,
            List<String> countries) {

        public StoryBundle {
            sources = sources == null ? List.of() : List.copyOf(sources);
            riskTags = riskTags == null ? List.of() : List.copyOf(riskTags);
            countries = countries == null ? List.of() : List.copyOf(countries);
        }

        public record Source(UUID newsId, String publisher, String url,
                OffsetDateTime publishedAt, boolean primary) {
        }
    }

    public record CompanyBundle(
            String companyId,
            String displayAlias,
            String orgType,
            String okfsName,
            String ageGradation,
            String ecoMark,
            String okvedCode,
            String okvedName,
            String workersGradation,
            String clType,
            Double revenue,
            String groupId,
            String city,
            String region,
            Boolean ftsFlag,
            List<String> ftsCountries,
            ClientMarks marks,
            List<Feature> features) {

        public CompanyBundle {
            ftsCountries = ftsCountries == null ? List.of() : List.copyOf(ftsCountries);
            features = features == null ? List.of() : List.copyOf(features);
        }

        public record Feature(String key, String value, String derivedFrom, String shortValue) {
        }
    }

    public record Counterparty(String counterpartyId, String okvedName, String direction,
            double sharePct, double payAmt) {
    }

    public record GroupMember(String companyId, String region, String okvedName) {
    }

    public record SimilarStory(UUID storyId, String title, OffsetDateTime publishedAt) {
    }

    public record BankViewData(boolean found, List<Insight.BankView.Statement> statements,
            String query) {

        public BankViewData {
            statements = statements == null ? List.of() : List.copyOf(statements);
        }
    }
}
