package ru.alfahack.agent.agent;

public record EvidenceRow(String fieldPath, String refType, String ref, String attribution) {

    public static final String FACT = "FACT";
    public static final String BANK_VIEW = "BANK_VIEW";
    public static final String AI_INFERENCE = "AI_INFERENCE";
}
