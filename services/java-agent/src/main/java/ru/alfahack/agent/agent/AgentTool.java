package ru.alfahack.agent.agent;

public interface AgentTool {

    String name();

    String run(AgentRunContext context);
}
