package ru.alfahack.agent.agent;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

import org.springframework.stereotype.Component;

@Component
public class ToolRegistry {

    private final Map<String, AgentTool> tools = new LinkedHashMap<>();

    public ToolRegistry(List<AgentTool> registered) {
        for (AgentTool tool : registered) {
            tools.put(tool.name(), tool);
        }
    }

    public List<String> names() {
        return List.copyOf(tools.keySet());
    }

    public Optional<AgentTool> byName(String name) {
        return Optional.ofNullable(tools.get(name));
    }

    public int size() {
        return tools.size();
    }
}
