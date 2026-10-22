package ru.alfahack.agent.domain;

import java.util.List;

public record ExtractedFacts(List<String> factsConfirmed, List<String> factsAssumed) {
}
