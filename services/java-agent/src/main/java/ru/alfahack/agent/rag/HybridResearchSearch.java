package ru.alfahack.agent.rag;

import java.util.ArrayList;
import java.util.List;
import java.util.Set;

import org.springframework.stereotype.Service;

import lombok.RequiredArgsConstructor;

import ru.alfahack.agent.cib.RiskTag;
import ru.alfahack.agent.config.AgentProperties;
import ru.alfahack.agent.domain.Insight;

@Service
@RequiredArgsConstructor
public class HybridResearchSearch {

    private static final double TAG_BOOST = 0.05;

    private final ResearchChunkStore store;
    private final EmbeddingPort embeddingPort;
    private final QueryComposer composer;
    private final AgentProperties props;

    public BankViewResult bankView(RagRequest request) {
        String query = composer.compose(request);
        Set<RiskTag> boostTags = composer.boostTags(request);
        var rag = props.rag();

        List<ScoredChunk> lexical = ranked(store.searchLexical(query, rag.candidateLimit()).stream()
                .filter(chunk -> chunk.score() >= rag.lexicalMinScore())
                .map(chunk -> boost(chunk, boostTags))
                .toList());

        boolean vectorUsed = false;
        List<ScoredChunk> vector = List.of();
        if (store.hasEmbeddings()) {
            var queryEmbedding = embeddingPort.embed(query);
            if (queryEmbedding.isPresent()) {
                vectorUsed = true;
                vector = ranked(store.searchVector(queryEmbedding.get(), rag.candidateLimit()).stream()
                        .filter(chunk -> chunk.score() >= rag.vectorMinScore())
                        .map(chunk -> boost(chunk, boostTags))
                        .toList());
            }
        }

        List<ScoredChunk> fused = RrfFuser.fuse(lexical, vector, rag.rrfK());
        if (fused.isEmpty()) {
            return BankViewResult.empty(query, vectorUsed);
        }

        List<ScoredChunk> top = fused.stream().limit(rag.maxStatements()).toList();
        List<Insight.BankView.Statement> statements = new ArrayList<>(top.size());
        for (ScoredChunk chunk : top) {
            statements.add(new Insight.BankView.Statement(
                    chunk.text(),
                    new Insight.BankView.StatementSource(chunk.document(), chunk.page(), chunk.chunkId())));
        }
        return new BankViewResult(new Insight.BankView(true, statements), top, query, vectorUsed);
    }

    private static List<ScoredChunk> ranked(List<ScoredChunk> chunks) {
        return chunks.stream()
                .sorted((x, y) -> {
                    int byScore = Double.compare(y.score(), x.score());
                    return byScore != 0 ? byScore : x.chunkId().compareTo(y.chunkId());
                })
                .toList();
    }

    private ScoredChunk boost(ScoredChunk chunk, Set<RiskTag> boostTags) {
        if (boostTags.isEmpty() || chunk.tags().isEmpty()) {
            return chunk;
        }
        long overlap = chunk.tags().stream().filter(boostTags::contains).count();
        if (overlap == 0) {
            return chunk;
        }
        return chunk.withScore(chunk.score() + TAG_BOOST * overlap, chunk.fromLexical(), chunk.fromVector());
    }
}
