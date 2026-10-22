package ru.alfahack.agent.domain;

import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import java.io.UncheckedIOException;

import org.junit.jupiter.api.Test;

import ru.alfahack.agent.config.JacksonConfig;

class InsightContractTest {

    @Test
    void parsesFrozenContractExample() throws IOException {
        Insight insight = readExample();

        assertThat(insight.companyId()).isEqualTo("U_417");
        assertThat(insight.confidence()).isEqualTo(Insight.Confidence.HIGH);
        assertThat(insight.story().title()).contains("РЖД");
        assertThat(insight.story().sources()).hasSize(1);
        assertThat(insight.story().sources().get(0).isPrimary()).isTrue();
        assertThat(insight.story().factsConfirmed()).hasSize(1);

        assertThat(insight.bankView().found()).isTrue();
        assertThat(insight.bankView().statements().get(0).source().page()).isEqualTo(7);
        assertThat(insight.bankView().statements().get(0).source().document())
                .isEqualTo("Тактический взгляд на 3-й квартал 2026 года");

        assertThat(insight.impactAnalysis().impactScore()).isEqualTo(-8);
        assertThat(insight.impactAnalysis().probability()).isEqualTo(0.7);
        assertThat(insight.impactAnalysis().firstOrderEffect().attribution())
                .isEqualTo(Insight.Attribution.FACT);
        assertThat(insight.impactAnalysis().secondOrderCompanyEffect().attribution())
                .isEqualTo(Insight.Attribution.AI_INFERENCE);
        assertThat(insight.impactAnalysis().secondOrderCompanyEffect().evidence()).hasSize(3);

        assertThat(insight.actionablePlaybook().alfaCibProductMatch().clientHasProduct()).isFalse();
        assertThat(insight.actionablePlaybook().alfaCibProductMatch().matchedBy())
                .containsExactly("FACTORING_MARK=0", "CL_PRODUCT_LOA_MARK=1");

        assertThat(insight.trace().tokensUsed()).isEqualTo(11840);
        assertThat(insight.trace().costRub()).isEqualTo(0.41);
    }

    @Test
    void roundTripsThroughSnakeCaseJson() throws IOException {
        Insight original = readExample();

        var tree = JacksonConfig.createMapper().valueToTree(original);
        assertThat(tree.has("insight_id")).as("имена полей в snake_case, как в контракте").isTrue();
        assertThat(tree.has("agent_run_id")).isTrue();
        assertThat(tree.path("impact_analysis").has("impact_score")).isTrue();

        Insight reread = JacksonConfig.createMapper().treeToValue(tree, Insight.class);
        assertThat(reread).isEqualTo(original);
    }

    private static Insight readExample() {
        try (var in = InsightContractTest.class.getResourceAsStream("/insight_contract_example.json")) {
            if (in == null) {
                throw new IllegalStateException("тестовый пример контракта не найден");
            }
            return JacksonConfig.createMapper().readValue(in, Insight.class);
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }
    }
}
