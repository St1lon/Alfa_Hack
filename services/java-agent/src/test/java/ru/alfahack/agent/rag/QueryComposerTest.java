package ru.alfahack.agent.rag;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;

import org.junit.jupiter.api.Test;

class QueryComposerTest {

    private final QueryComposer composer = new QueryComposer();

    @Test
    void debtSensitivityAddsRateTerms() {
        var request = new RagRequest(
                "ЦБ сохранил ключевую ставку",
                null, List.of(), null,
                new CompanySnapshot("Металлургия", List.of(), true, false, false));

        assertThat(composer.compose(request)).contains("ключевая ставка");
    }

    @Test
    void exporterQueryKeepsCountryAndIndustry() {
        var request = new RagRequest(
                "Пошлины на экспорт",
                null, List.of(), null,
                new CompanySnapshot("Металлургия", List.of("КИТАЙ", "Турция"), false, false, false));

        String query = composer.compose(request);
        assertThat(query).contains("Металлургия").contains("КИТАЙ").contains("Турция");
    }

    @Test
    void cappedToMaxLength() {
        var request = new RagRequest(
                "т".repeat(5000),
                "с".repeat(5000),
                List.of("факт один очень длинный".repeat(50), "факт два"),
                null,
                new CompanySnapshot(null, List.of(), false, false, false));

        assertThat(composer.compose(request)).hasSizeLessThanOrEqualTo(500);
    }

    @Test
    void factsAndSummaryIncluded() {
        var request = new RagRequest(
                "Заголовок",
                "Краткое описание события",
                List.of("Тариф индексируется на 10%"),
                null,
                CompanySnapshot.empty());

        assertThat(composer.compose(request))
                .contains("Заголовок")
                .contains("Краткое описание события")
                .contains("Тариф индексируется на 10%");
    }
}
