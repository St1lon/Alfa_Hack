package ru.alfahack.agent.agent.tools;

import java.util.List;

import org.springframework.stereotype.Component;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.agent.AgentTool;

@Component
public class GetCounterpartyExposureTool implements AgentTool {

    @Override
    public String name() {
        return "get_counterparty_exposure";
    }

    @Override
    public String run(AgentRunContext ctx) {
        List<AgentRunContext.Counterparty> top = ctx.counterparties().stream().limit(10).toList();
        if (top.isEmpty()) {
            return "{\"counterparties\":[]}";
        }
        ctx.registerCounterparties(top);
        StringBuilder sb = new StringBuilder(512);
        sb.append("{\"counterparties\":[");
        for (int i = 0; i < top.size(); i++) {
            AgentRunContext.Counterparty c = top.get(i);
            if (i > 0) {
                sb.append(',');
            }
            sb.append("{\"direction\":\"").append(c.direction())
                    .append("\",\"industry\":\"").append(c.okvedName())
                    .append("\",\"share_pct\":").append(Math.round(c.sharePct() * 10) / 10.0)
                    .append(",\"citation\":\"feat:counterparty_").append(i + 1).append("\"}");
        }
        sb.append("]}");
        return sb.toString();
    }
}
