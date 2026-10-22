package ru.alfahack.agent.agent.tools;

import java.util.stream.Collectors;

import org.springframework.stereotype.Component;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.agent.AgentTool;

@Component
public class GetGroupContextTool implements AgentTool {

    @Override
    public String name() {
        return "get_group_context";
    }

    @Override
    public String run(AgentRunContext ctx) {
        if (ctx.groupMembers().isEmpty()) {
            return "{\"group\":null}";
        }
        String members = ctx.groupMembers().stream()
                .map(m -> "{\"region\":\"" + m.region() + "\",\"industry\":\"" + m.okvedName() + "\"}")
                .collect(Collectors.joining(","));
        return "{\"group_size\":" + (ctx.groupMembers().size() + 1) + ",\"members\":[" + members + "]}";
    }
}
