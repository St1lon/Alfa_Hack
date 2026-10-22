package ru.alfahack.agent.cib;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Optional;
import java.util.Set;

import org.junit.jupiter.api.Test;

import ru.alfahack.agent.config.JacksonConfig;
import ru.alfahack.agent.domain.Insight;

class CibMatcherTest {

    private final CibMatcher matcher = new CibMatcher(new CibCatalog(JacksonConfig.createMapper()));

    @Test
    void exporterWithFxExposureRiskGetsForward() {
        var match = matcher.match(
                Set.of(RiskTag.FX_RISK, RiskTag.EXPORT),
                new ClientMarks(false, false, false, false, false, false, false));

        assertThat(match).isPresent();
        assertThat(match.get().productName()).isEqualTo("Валютный форвард");
        assertThat(match.get().riskTags()).containsExactly("fx_risk", "export");
        assertThat(match.get().clientHasProduct()).isFalse();
        assertThat(match.get().matchedBy()).containsExactly("risk_tag=fx_risk", "risk_tag=export");
    }

    @Test
    void supplyChainExportWithoutFactoringSuggestsInternationalFactoring() {
        var match = matcher.match(
                Set.of(RiskTag.LIQUIDITY_GAP, RiskTag.EXPORT, RiskTag.SUPPLY_CHAIN),
                new ClientMarks(false, false, true, false, false, false, false));

        assertThat(match).isPresent();
        assertThat(match.get().productName()).isEqualTo("Международный факторинг");
        assertThat(match.get().clientHasProduct()).isFalse();
        assertThat(match.get().matchedBy())
                .containsExactly("FACTORING_MARK=0", "risk_tag=liquidity_gap", "risk_tag=export",
                        "risk_tag=supply_chain");
    }

    @Test
    void creditLineWinsOverBondLoanByCatalogOrderAndNotesExistingProduct() {
        var match = matcher.match(
                Set.of(RiskTag.RATE_RISK, RiskTag.LIQUIDITY_GAP, RiskTag.CAPEX),
                new ClientMarks(false, false, true, false, false, false, false));

        assertThat(match).isPresent();
        assertThat(match.get().productName()).isEqualTo("Кредитная линия на пополнение оборотных средств");
        assertThat(match.get().clientHasProduct()).isTrue();
        assertThat(match.get().rationale()).contains("уже есть");
        assertThat(match.get().matchedBy()).contains("CL_PRODUCT_LOA_MARK=1");
    }

    @Test
    void noOverlapMeansNoMatch() {
        Optional<Insight.ActionablePlaybook.AlfaCibProductMatch> match =
                matcher.match(Set.of(), new ClientMarks(true, true, true, true, true, true, true));

        assertThat(match).isEmpty();
    }
}
