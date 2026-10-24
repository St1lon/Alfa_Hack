package ru.alfahack.agent.agent;

import java.util.List;

public record QaOutput(String answer, List<String> citations) {

    public QaOutput {
        citations = citations == null ? List.of() : List.copyOf(citations);
    }
}
