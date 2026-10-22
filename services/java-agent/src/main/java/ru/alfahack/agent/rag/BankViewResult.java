package ru.alfahack.agent.rag;

import java.util.List;

import ru.alfahack.agent.domain.Insight;

public record BankViewResult(
        Insight.BankView view,
        List<ScoredChunk> chunks,
        String query,
        boolean vectorUsed) {

    public BankViewResult {
        chunks = chunks == null ? List.of() : List.copyOf(chunks);
    }

    public static BankViewResult empty(String query, boolean vectorUsed) {
        return new BankViewResult(new Insight.BankView(false, List.of()), List.of(), query, vectorUsed);
    }
}
