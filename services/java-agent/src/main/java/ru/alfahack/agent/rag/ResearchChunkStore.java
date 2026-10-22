package ru.alfahack.agent.rag;

import java.util.List;

public interface ResearchChunkStore {

    List<ScoredChunk> searchLexical(String query, int limit);

    List<ScoredChunk> searchVector(float[] embedding, int limit);

    boolean hasEmbeddings();
}
