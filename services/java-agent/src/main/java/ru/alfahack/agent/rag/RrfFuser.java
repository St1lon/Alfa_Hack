package ru.alfahack.agent.rag;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;

import ru.alfahack.agent.cib.RiskTag;

public final class RrfFuser {

    private RrfFuser() {
    }

    public static List<ScoredChunk> fuse(List<ScoredChunk> lexical, List<ScoredChunk> vector, int k) {
        Map<UUID, Acc> acc = new HashMap<>();
        accumulate(acc, lexical, k, true);
        accumulate(acc, vector, k, false);

        List<ScoredChunk> fused = new ArrayList<>(acc.size());
        for (Acc a : acc.values()) {
            fused.add(new ScoredChunk(
                    a.first.chunkId(), a.first.document(), a.first.page(), a.first.text(),
                    a.score, a.fromLexical, a.fromVector, a.tags));
        }
        fused.sort((x, y) -> {
            int byScore = Double.compare(y.score(), x.score());
            return byScore != 0 ? byScore : x.chunkId().compareTo(y.chunkId());
        });
        return List.copyOf(fused);
    }

    private static void accumulate(Map<UUID, Acc> acc, List<ScoredChunk> channel, int k, boolean lexical) {
        for (int rank = 0; rank < channel.size(); rank++) {
            ScoredChunk chunk = channel.get(rank);
            Acc a = acc.computeIfAbsent(chunk.chunkId(), id -> new Acc(chunk));
            a.score += 1.0 / (k + rank + 1);
            a.fromLexical |= lexical;
            a.fromVector |= !lexical;
            a.tags.addAll(chunk.tags());
        }
    }

    private static final class Acc {
        final ScoredChunk first;
        final Set<RiskTag> tags = new HashSet<>();
        double score;
        boolean fromLexical;
        boolean fromVector;

        Acc(ScoredChunk first) {
            this.first = first;
        }
    }
}
