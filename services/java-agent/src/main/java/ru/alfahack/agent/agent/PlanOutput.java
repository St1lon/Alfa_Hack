package ru.alfahack.agent.agent;

import java.util.List;

public record PlanOutput(List<String> tools, String note) {

    public PlanOutput {
        tools = tools == null ? List.of() : List.copyOf(tools);
    }
}
