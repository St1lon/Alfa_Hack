package ru.alfahack.agent.agent;

import java.util.UUID;

public interface ContextLoader {

    AgentRunContext load(UUID storyId, String companyId);
}
