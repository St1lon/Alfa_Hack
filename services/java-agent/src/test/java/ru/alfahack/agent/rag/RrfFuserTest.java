package ru.alfahack.agent.rag;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Set;
import java.util.UUID;

import org.junit.jupiter.api.Test;

import ru.alfahack.agent.cib.RiskTag;

class RrfFuserTest {

    private static final int K = 60;

    @Test
    void chunkFromBothChannelsOutranksSingleChannel() {
        UUID both = UUID.nameUUIDFromBytes("both".getBytes());
        UUID lexicalOnly = UUID.nameUUIDFromBytes("lexical".getBytes());

        List<ScoredChunk> fused = RrfFuser.fuse(
                List.of(chunk(lexicalOnly, 0.9), chunk(both, 0.5)),
                List.of(chunk(both, 0.4)),
                K);

        assertThat(fused.get(0).chunkId()).isEqualTo(both);
        assertThat(fused.get(0).fromLexical()).isTrue();
        assertThat(fused.get(0).fromVector()).isTrue();
        assertThat(fused.get(1).chunkId()).isEqualTo(lexicalOnly);
    }

    @Test
    void rankPositionMattersInsideChannel() {
        UUID first = UUID.nameUUIDFromBytes("first".getBytes());
        UUID second = UUID.nameUUIDFromBytes("second".getBytes());

        List<ScoredChunk> fused = RrfFuser.fuse(
                List.of(chunk(first, 0.2), chunk(second, 0.9)),
                List.of(),
                K);

        assertThat(fused.get(0).chunkId()).isEqualTo(first);
    }

    @Test
    void tieBrokenByChunkIdForDeterminism() {
        UUID a = UUID.nameUUIDFromBytes("a".getBytes());
        UUID b = UUID.nameUUIDFromBytes("b".getBytes());
        if (b.compareTo(a) < 0) {
            UUID t = a;
            a = b;
            b = t;
        }

        List<ScoredChunk> fused = RrfFuser.fuse(
                List.of(chunk(a, 0.5), chunk(b, 0.5)),
                List.of(),
                K);

        assertThat(fused.get(0).chunkId()).isEqualTo(a);
        assertThat(fused.get(1).chunkId()).isEqualTo(b);
    }

    @Test
    void tagsUnionedAcrossChannels() {
        UUID id = UUID.nameUUIDFromBytes("id".getBytes());

        List<ScoredChunk> fused = RrfFuser.fuse(
                List.of(new ScoredChunk(id, "doc", 1, "text", 0.5, false, false, Set.of(RiskTag.RATE_RISK))),
                List.of(new ScoredChunk(id, "doc", 1, "text", 0.4, false, false, Set.of(RiskTag.EXPORT))),
                K);

        assertThat(fused.get(0).tags()).containsExactlyInAnyOrder(RiskTag.RATE_RISK, RiskTag.EXPORT);
    }

    @Test
    void emptyChannelsGiveEmptyResult() {
        assertThat(RrfFuser.fuse(List.of(), List.of(), K)).isEmpty();
    }

    private static ScoredChunk chunk(UUID id, double score) {
        return new ScoredChunk(id, "doc", 1, "text", score, false, false, Set.of());
    }
}
