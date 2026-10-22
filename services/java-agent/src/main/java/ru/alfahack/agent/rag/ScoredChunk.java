package ru.alfahack.agent.rag;

import java.util.Set;
import java.util.UUID;

import ru.alfahack.agent.cib.RiskTag;

public record ScoredChunk(
        UUID chunkId,
        String document,
        int page,
        String text,
        double score,
        boolean fromLexical,
        boolean fromVector,
        Set<RiskTag> tags) {

    public ScoredChunk {
        tags = tags == null ? Set.of() : Set.copyOf(tags);
    }

    public ScoredChunk withScore(double newScore, boolean lexical, boolean vector) {
        return new ScoredChunk(chunkId, document, page, text, newScore, lexical, vector, tags);
    }
}
