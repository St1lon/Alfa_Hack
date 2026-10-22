package ru.alfahack.agent.llm;

public enum Task {

    CLASSIFY("classify"),

    EXTRACT("extract"),

    ANALYZE("analyze"),

    REASON("reason"),

    CRITIC("critic"),

    RELATIONS("relations");

    private final String code;

    Task(String code) {
        this.code = code;
    }

    public String code() {
        return code;
    }
}
