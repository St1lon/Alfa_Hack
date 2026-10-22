package ru.alfahack.agent.rag;

import java.util.HashSet;
import java.util.LinkedHashSet;
import java.util.Set;

import org.springframework.stereotype.Component;

import ru.alfahack.agent.cib.RiskTag;

@Component
public class QueryComposer {

    private static final int MAX_QUERY_CHARS = 500;

    public String compose(RagRequest request) {
        LinkedHashSet<String> parts = new LinkedHashSet<>();
        addPart(parts, truncate(request.storyTitle(), 160));
        addPart(parts, truncate(request.storySummary(), 200));
        request.facts().stream().limit(2).forEach(fact -> addPart(parts, truncate(fact, 120)));
        addPart(parts, request.company().industryLabel());
        request.company().ftsCountries().stream().limit(3).forEach(parts::add);

        if (request.company().debtSensitivity()) {
            addPart(parts, "ключевая ставка стоимость заёмного финансирования");
        }
        if (request.company().liquiditySensitivity()) {
            addPart(parts, "оборотные средства кассовый разрыв ликвидность");
        }
        if (request.company().payrollSensitivity()) {
            addPart(parts, "фонд оплаты труда");
        }

        String query = String.join(" ", parts).strip();
        return query.length() > MAX_QUERY_CHARS ? query.substring(0, MAX_QUERY_CHARS).strip() : query;
    }

    public Set<RiskTag> boostTags(RagRequest request) {
        Set<RiskTag> tags = new HashSet<>(request.riskTags());
        if (request.company().debtSensitivity()) {
            tags.add(RiskTag.RATE_RISK);
        }
        if (request.company().liquiditySensitivity()) {
            tags.add(RiskTag.LIQUIDITY_GAP);
        }
        return Set.copyOf(tags);
    }

    private static void addPart(Set<String> parts, String part) {
        if (part != null && !part.isBlank()) {
            parts.add(part.strip());
        }
    }

    private static String truncate(String value, int limit) {
        if (value == null) {
            return null;
        }
        return value.length() <= limit ? value : value.substring(0, limit);
    }
}
