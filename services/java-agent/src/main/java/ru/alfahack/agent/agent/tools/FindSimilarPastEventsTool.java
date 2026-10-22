package ru.alfahack.agent.agent.tools;

import java.util.stream.Collectors;

import org.springframework.stereotype.Component;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.agent.AgentTool;

@Component
public class FindSimilarPastEventsTool implements AgentTool {

    @Override
    public String name() {
        return "find_similar_past_events";
    }

    @Override
    public String run(AgentRunContext ctx) {
        if (ctx.similarStories().isEmpty()) {
            return "{\"similar\":[]}";
        }
        String items = ctx.similarStories().stream()
                .map(s -> "{\"title\":\"" + s.title().replace("\"", "'") + "\"}")
                .collect(Collectors.joining(","));
        return "{\"similar\":[" + items + "]}";
    }
}
