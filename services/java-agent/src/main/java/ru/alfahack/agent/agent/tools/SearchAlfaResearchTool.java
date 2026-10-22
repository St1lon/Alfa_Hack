package ru.alfahack.agent.agent.tools;

import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;

import org.springframework.stereotype.Component;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.agent.AgentTool;
import ru.alfahack.agent.rag.BankViewResult;
import ru.alfahack.agent.rag.CompanySnapshot;
import ru.alfahack.agent.rag.HybridResearchSearch;
import ru.alfahack.agent.rag.RagRequest;

@Component
public class SearchAlfaResearchTool implements AgentTool {

    private final HybridResearchSearch search;

    public SearchAlfaResearchTool(HybridResearchSearch search) {
        this.search = search;
    }

    @Override
    public String name() {
        return "search_alfa_research";
    }

    @Override
    public String run(AgentRunContext ctx) {
        var company = ctx.company();
        var story = ctx.story();
        BankViewResult result = search.bankView(new RagRequest(
                story.title(),
                story.summary(),
                List.of(),
                Set.copyOf(story.riskTags()),
                new CompanySnapshot(
                        company.okvedName(),
                        company.ftsCountries(),
                        company.marks().loa() || company.marks().lea(),
                        company.marks().factoring() || company.marks().gar(),
                        company.marks().zpp())));
        ctx.setBankViewData(new AgentRunContext.BankViewData(
                result.view().found(), result.view().statements(), result.query()));

        if (!result.view().found()) {
            return "{\"found\":false,\"note\":\"позиция банка по теме не найдена\"}";
        }
        String statements = result.chunks().stream()
                .map(c -> "{\"document\":\"" + c.document() + "\",\"page\":" + c.page()
                        + ",\"citation\":\"chunk:" + c.chunkId() + "\"}")
                .collect(Collectors.joining(","));
        return "{\"found\":true,\"statements\":[" + statements + "]}";
    }
}
