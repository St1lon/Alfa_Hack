package ru.alfahack.agent.cib;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.Set;

import org.springframework.stereotype.Component;

import lombok.RequiredArgsConstructor;

import ru.alfahack.agent.domain.Insight;

@Component
@RequiredArgsConstructor
public class CibMatcher {

    private final CibCatalog catalog;

    public Optional<Insight.ActionablePlaybook.AlfaCibProductMatch> match(Set<RiskTag> eventTags,
            ClientMarks marks) {
        if (eventTags == null || eventTags.isEmpty()) {
            return Optional.empty();
        }

        CibProduct best = null;
        List<RiskTag> bestFired = List.of();
        for (CibProduct product : catalog.all()) {
            List<RiskTag> fired = product.riskTags().stream()
                    .filter(eventTags::contains)
                    .toList();
            if (fired.size() > bestFired.size()) {
                best = product;
                bestFired = fired;
            }
        }
        if (best == null) {
            return Optional.empty();
        }

        boolean clientHas = marks.byMark(best.clientMark());

        List<String> matchedBy = new ArrayList<>();
        if (best.clientMark() != null) {
            matchedBy.add(marks.describe(best.clientMark()));
        }
        bestFired.stream()
                .map(tag -> "risk_tag=" + tag.code())
                .forEach(matchedBy::add);

        String rationale = best.description()
                + (clientHas
                        ? " У клиента этот продукт уже есть — целесообразно обсудить расширение лимита."
                        : "");

        return Optional.of(new Insight.ActionablePlaybook.AlfaCibProductMatch(
                best.name(),
                bestFired.stream().map(RiskTag::code).toList(),
                rationale,
                clientHas,
                matchedBy));
    }
}
