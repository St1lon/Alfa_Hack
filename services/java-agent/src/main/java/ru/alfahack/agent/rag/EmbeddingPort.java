package ru.alfahack.agent.rag;

import java.util.Optional;

public interface EmbeddingPort {

    Optional<float[]> embed(String text);
}
