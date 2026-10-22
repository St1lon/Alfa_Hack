package ru.alfahack.agent.repository;

import java.util.List;
import java.util.UUID;

import org.springframework.stereotype.Repository;

import ru.alfahack.agent.agent.AgentRunContext;
import ru.alfahack.agent.agent.ContextLoader;

@Repository
public class JdbcContextLoader implements ContextLoader {

    private final StoryRepository stories;
    private final CompanyRepository companies;

    public JdbcContextLoader(StoryRepository stories, CompanyRepository companies) {
        this.stories = stories;
        this.companies = companies;
    }

    @Override
    public AgentRunContext load(UUID storyId, String companyId) {
        var story = stories.loadBundle(storyId);
        var company = companies.loadBundle(companyId);
        List<AgentRunContext.Counterparty> counterparties = companies.loadCounterparties(companyId, 5);
        List<AgentRunContext.GroupMember> group = companies.loadGroupMembers(company.groupId(), companyId, 10);
        List<AgentRunContext.SimilarStory> similar = stories.findSimilar(storyId, story.title(), 3);
        var personalization = stories.loadPersonalization(storyId, companyId);
        return new AgentRunContext(storyId, companyId, story, company, counterparties, group, similar,
                personalization.relevanceScore(), personalization.matchReasons());
    }
}
