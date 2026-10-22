package ru.alfahack.agent.rag;

import java.util.List;
import java.util.Set;

import ru.alfahack.agent.cib.RiskTag;

public record RagRequest(
        String storyTitle,
        String storySummary,
        List<String> facts,
        Set<RiskTag> riskTags,
        CompanySnapshot company) {

    public RagRequest {
        facts = facts == null ? List.of() : List.copyOf(facts);
        riskTags = riskTags == null ? Set.of() : Set.copyOf(riskTags);
        company = company == null ? CompanySnapshot.empty() : company;
    }
}
