package ru.alfahack.agent.rag;

import java.util.Optional;

import org.springframework.stereotype.Component;

@Component
public class NoopEmbeddingClient implements EmbeddingPort {

    @Override
    public Optional<float[]> embed(String text) {
        return Optional.empty();
    }
}
