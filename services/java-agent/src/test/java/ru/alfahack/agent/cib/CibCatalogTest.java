package ru.alfahack.agent.cib;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

import ru.alfahack.agent.config.JacksonConfig;

class CibCatalogTest {

    private final CibCatalog catalog = new CibCatalog(JacksonConfig.createMapper());

    @Test
    void hasAtLeastTenProductsAsPlanned() {
        assertThat(catalog.all()).hasSizeGreaterThanOrEqualTo(10);
        assertThat(catalog.all()).allSatisfy(product -> {
            assertThat(product.name()).isNotBlank();
            assertThat(product.description()).isNotBlank();
            assertThat(product.riskTags()).isNotEmpty();
        });
    }

    @Test
    void findsProductByCode() {
        assertThat(catalog.byCode("fx_forward")).isPresent();
        assertThat(catalog.byCode("missing")).isEmpty();
    }

    @Test
    void clientMarksReflectDatasetFlags() {
        ClientMarks marks = new ClientMarks(false, true, true, false, false, false, false);

        assertThat(marks.byMark("CL_PRODUCT_GAR_MARK")).isTrue();
        assertThat(marks.byMark("CL_PRODUCT_LOA_MARK")).isTrue();
        assertThat(marks.byMark("FACTORING_MARK")).isFalse();
        assertThat(marks.byMark("UNKNOWN_MARK")).isFalse();
        assertThat(marks.describe("FACTORING_MARK")).isEqualTo("FACTORING_MARK=0");
    }
}
