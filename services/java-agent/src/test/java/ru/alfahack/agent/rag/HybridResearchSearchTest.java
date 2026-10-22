package ru.alfahack.agent.rag;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;

import org.junit.jupiter.api.Test;

import ru.alfahack.agent.cib.RiskTag;
import ru.alfahack.agent.config.AgentProperties;

class HybridResearchSearchTest {

    @Test
    void lexicalHitBecomesBankViewWithDocumentAndPage() {
        ResearchChunkStore store = mock(ResearchChunkStore.class);
        when(store.searchLexical(anyString(), anyInt())).thenReturn(List.of(
                chunk("c1", 0.42, Set.of()),
                chunk("c2", 0.31, Set.of()),
                chunk("c3", 0.22, Set.of())));
        when(store.hasEmbeddings()).thenReturn(false);
        HybridResearchSearch search = search(store, text -> Optional.empty());

        BankViewResult result = search.bankView(request(Set.of()));

        assertThat(result.vectorUsed()).isFalse();
        assertThat(result.view().found()).isTrue();
        assertThat(result.view().statements()).hasSize(3);
        assertThat(result.view().statements().get(0).text()).isEqualTo("текст c1");
        assertThat(result.view().statements().get(0).source().document()).isEqualTo("Тактический взгляд");
        assertThat(result.view().statements().get(0).source().page()).isEqualTo(7);
        assertThat(result.view().statements().get(0).source().chunkId())
                .isEqualTo(UUID.nameUUIDFromBytes("c1".getBytes()));
    }

    @Test
    void nothingFoundIsHonestNotFound() {
        ResearchChunkStore store = mock(ResearchChunkStore.class);
        when(store.searchLexical(anyString(), anyInt())).thenReturn(List.of());
        when(store.hasEmbeddings()).thenReturn(false);
        HybridResearchSearch search = search(store, text -> Optional.empty());

        BankViewResult result = search.bankView(request(Set.of()));

        assertThat(result.view().found()).isFalse();
        assertThat(result.view().statements()).isEmpty();
        assertThat(result.chunks()).isEmpty();
    }

    @Test
    void channelThresholdFiltersWeakLexicalMatches() {
        ResearchChunkStore store = mock(ResearchChunkStore.class);
        when(store.searchLexical(anyString(), anyInt())).thenReturn(List.of(chunk("weak", 0.05, Set.of())));
        when(store.hasEmbeddings()).thenReturn(false);
        HybridResearchSearch search = search(store, text -> Optional.empty());

        assertThat(search.bankView(request(Set.of())).view().found()).isFalse();
    }

    @Test
    void vectorChannelFusesWhenEmbeddingsReady() {
        ResearchChunkStore store = mock(ResearchChunkStore.class);
        UUID shared = UUID.nameUUIDFromBytes("shared".getBytes());
        when(store.searchLexical(anyString(), anyInt())).thenReturn(List.of(
                chunk("shared", 0.30, Set.of()),
                chunk("other", 0.28, Set.of())));
        when(store.hasEmbeddings()).thenReturn(true);
        when(store.searchVector(any(), anyInt())).thenReturn(List.of(
                new ScoredChunk(shared, "Тактический взгляд", 7, "текст shared", 0.8, false, false, Set.of())));
        HybridResearchSearch search = search(store, text -> Optional.of(new float[] {0.1f, 0.2f}));

        BankViewResult result = search.bankView(request(Set.of()));

        assertThat(result.vectorUsed()).isTrue();
        assertThat(result.view().found()).isTrue();
        assertThat(result.chunks().get(0).chunkId()).isEqualTo(shared);
        assertThat(result.chunks().get(0).fromLexical()).isTrue();
        assertThat(result.chunks().get(0).fromVector()).isTrue();
    }

    @Test
    void tagBoostPrioritizesRelevantChunkOnEqualScores() {
        ResearchChunkStore store = mock(ResearchChunkStore.class);
        when(store.searchLexical(anyString(), anyInt())).thenReturn(List.of(
                chunk("plain", 0.30, Set.of()),
                chunk("tagged", 0.30, Set.of(RiskTag.RATE_RISK))));
        when(store.hasEmbeddings()).thenReturn(false);
        HybridResearchSearch search = search(store, text -> Optional.empty());

        BankViewResult result = search.bankView(request(Set.of(RiskTag.RATE_RISK)));

        assertThat(result.chunks().get(0).text()).isEqualTo("текст tagged");
    }

    @Test
    void maxStatementsLimitsBankView() {
        ResearchChunkStore store = mock(ResearchChunkStore.class);
        when(store.searchLexical(anyString(), anyInt())).thenReturn(List.of(
                chunk("c1", 0.5, Set.of()),
                chunk("c2", 0.4, Set.of()),
                chunk("c3", 0.3, Set.of()),
                chunk("c4", 0.2, Set.of())));
        when(store.hasEmbeddings()).thenReturn(false);
        HybridResearchSearch search = search(store, text -> Optional.empty());

        assertThat(search.bankView(request(Set.of())).view().statements()).hasSize(3);
    }

    private static HybridResearchSearch search(ResearchChunkStore store, EmbeddingPort embedding) {
        return new HybridResearchSearch(store, embedding, new QueryComposer(), props());
    }

    private static AgentProperties props() {
        return new AgentProperties(
                new AgentProperties.Gateway("http://gateway.test", "", java.time.Duration.ofSeconds(1),
                        java.time.Duration.ofSeconds(1), 3, true),
                new AgentProperties.Rag(0.15, 0.5, 60, 3, 20),
                new AgentProperties.Budget(6, 2, 20000),
                "v0.1", "java", new AgentProperties.Selfcheck(false));
    }

    private static RagRequest request(Set<RiskTag> riskTags) {
        return new RagRequest("ЦБ сохранил ключевую ставку", null, List.of(), riskTags,
                new CompanySnapshot("Металлургия", List.of(), true, false, false));
    }

    private static ScoredChunk chunk(String key, double score, Set<RiskTag> tags) {
        return new ScoredChunk(UUID.nameUUIDFromBytes(key.getBytes()), "Тактический взгляд", 7,
                "текст " + key, score, false, false, tags);
    }

    private static float[] any() {
        return org.mockito.ArgumentMatchers.any(float[].class);
    }
}
