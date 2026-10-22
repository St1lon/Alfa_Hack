package ru.alfahack.agent.cib;

import java.util.List;

public record CibProduct(
        String code,
        String name,
        String description,
        List<RiskTag> riskTags,
        String clientMark) {
}
