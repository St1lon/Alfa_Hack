package ru.alfahack.agent.agent.tools;

import java.util.stream.Collectors;

import org.springframework.stereotype.Component;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.agent.AgentTool;
import ru.alfahack.agent.cib.CibCatalog;

@Component
public class ListCibProductsTool implements AgentTool {

    private final CibCatalog catalog;

    public ListCibProductsTool(CibCatalog catalog) {
        this.catalog = catalog;
    }

    @Override
    public String name() {
        return "list_cib_products";
    }

    @Override
    public String run(AgentRunContext ctx) {
        String items = catalog.all().stream()
                .map(p -> "{\"name\":\"" + p.name() + "\",\"risk_tags\":["
                        + p.riskTags().stream().map(t -> '"' + t.code() + '"').collect(Collectors.joining(","))
                        + "]}")
                .collect(Collectors.joining(","));
        return "{\"products\":[" + items + "]}";
    }
}
