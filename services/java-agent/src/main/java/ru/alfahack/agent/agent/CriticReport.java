package ru.alfahack.agent.agent;

import java.util.List;

public record CriticReport(boolean passed, List<Failure> failures) {

    public CriticReport {
        failures = failures == null ? List.of() : List.copyOf(failures);
    }

    public record Failure(String check, String detail) {
    }
}
