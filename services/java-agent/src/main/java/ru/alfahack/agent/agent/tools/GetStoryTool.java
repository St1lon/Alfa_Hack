package ru.alfahack.agent.agent.tools;

import java.util.stream.Collectors;

import org.springframework.stereotype.Component;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.agent.AgentTool;

@Component
public class GetStoryTool implements AgentTool {

    @Override
    public String name() {
        return "get_story";
    }

    @Override
    public String run(AgentRunContext ctx) {
        var story = ctx.story();
        String sources = story.sources().stream()
                .map(s -> "{\"publisher\":\"" + s.publisher() + "\",\"citation\":\"news:" + s.newsId()
                        + "\",\"primary\":" + s.primary() + "}")
                .collect(Collectors.joining(","));
        String tags = story.riskTags().stream().map(t -> '"' + t.code() + '"').collect(Collectors.joining(","));
        String countries = story.countries().stream().map(c -> '"' + c + '"').collect(Collectors.joining(","));
        return "{\"title\":\"" + escape(story.title()) + "\",\"summary\":\"" + escape(story.summary())
                + "\",\"risk_tags\":[" + tags + "],\"countries\":[" + countries
                + "],\"sources\":[" + sources + "]}";
    }

    private static String escape(String value) {
        return value == null ? "" : value.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", " ");
    }
}
