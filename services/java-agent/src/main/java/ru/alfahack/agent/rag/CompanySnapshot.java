package ru.alfahack.agent.rag;

import java.util.List;

public record CompanySnapshot(
        String industryLabel,
        List<String> ftsCountries,
        boolean debtSensitivity,
        boolean liquiditySensitivity,
        boolean payrollSensitivity) {

    public CompanySnapshot {
        ftsCountries = ftsCountries == null ? List.of() : List.copyOf(ftsCountries);
    }

    public static CompanySnapshot empty() {
        return new CompanySnapshot(null, List.of(), false, false, false);
    }
}
