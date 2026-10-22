package ru.alfahack.agent.cib;

import java.util.Optional;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;

public enum RiskTag {

    RATE_RISK("rate_risk"),
    FX_RISK("fx_risk"),
    LIQUIDITY_GAP("liquidity_gap"),
    SUPPLY_CHAIN("supply_chain"),
    EXPORT("export"),
    IMPORT("import"),
    CAPEX("capex");

    private final String code;

    RiskTag(String code) {
        this.code = code;
    }

    @JsonValue
    public String code() {
        return code;
    }

    @JsonCreator
    static RiskTag fromCode(String code) {
        for (RiskTag tag : values()) {
            if (tag.code.equals(code)) {
                return tag;
            }
        }
        throw new IllegalArgumentException("неизвестный тег риска: " + code);
    }

    public static Optional<RiskTag> parseLenient(String code) {
        for (RiskTag tag : values()) {
            if (tag.code.equals(code)) {
                return Optional.of(tag);
            }
        }
        return Optional.empty();
    }
}
