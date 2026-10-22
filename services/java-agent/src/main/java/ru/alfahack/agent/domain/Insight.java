package ru.alfahack.agent.domain;

import java.time.OffsetDateTime;
import java.util.List;
import java.util.UUID;

public record Insight(
        UUID insightId,
        String companyId,
        String modelVersion,
        UUID agentRunId,
        OffsetDateTime generatedAt,
        Confidence confidence,
        StoryBlock story,
        Personalization personalization,
        BankView bankView,
        ImpactBlock impactAnalysis,
        ActionablePlaybook actionablePlaybook,
        TraceBlock trace) {

    public enum Confidence { HIGH, MEDIUM, LOW }

    public enum Attribution { FACT, BANK_VIEW, AI_INFERENCE }

    public record StoryBlock(
            UUID storyId,
            String title,
            Integer sourcesCount,
            List<Source> sources,
            List<String> factsConfirmed,
            List<String> factsAssumed) {
    }

    public record Source(
            UUID newsId,
            String publisher,
            String url,
            OffsetDateTime publishedAt,
            Boolean isPrimary) {
    }

    public record Personalization(
            Double relevanceScore,
            List<MatchReason> matchReasons,
            List<String> companyFeaturesUsed) {
    }

    public record MatchReason(String feature, String label, Double weight) {
    }

    public record BankView(boolean found, List<Statement> statements) {

        public record Statement(String text, StatementSource source) {
        }

        public record StatementSource(String document, Integer page, UUID chunkId) {
        }
    }

    public record ImpactBlock(
            String type,
            String severity,
            Integer impactScore,
            Double probability,
            String timeHorizon,
            List<String> targetPersonas,
            Effect firstOrderEffect,
            Effect secondOrderCompanyEffect) {
    }

    public record Effect(String text, Attribution attribution, List<EvidenceRef> evidence) {
    }

    public record EvidenceRef(String type, String ref) {
    }

    public record ActionablePlaybook(
            List<PlaybookAction> whatToDo,
            List<String> whatToWatch,
            List<String> risksToConsider,
            List<String> questionsForTeam,
            AlfaCibProductMatch alfaCibProductMatch) {

        public record AlfaCibProductMatch(
                String productName,
                List<String> riskTags,
                String rationale,
                Boolean clientHasProduct,
                List<String> matchedBy) {
        }
    }

    public record PlaybookAction(String text, Attribution attribution, List<EvidenceRef> evidence) {
    }

    public record TraceBlock(
            Integer stepsCount,
            List<String> toolsCalled,
            Integer criticIterations,
            Long tokensUsed,
            Double costRub) {
    }
}
