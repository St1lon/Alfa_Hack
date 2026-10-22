package ru.alfahack.agent.agent;

import java.util.List;

public record MarketDraft(FirstOrder firstOrder) {

    public MarketDraft {
        firstOrder = firstOrder == null
                ? new FirstOrder("Не сформулирован эффект для рынка", List.of())
                : firstOrder;
    }

    public record FirstOrder(String text, List<Driver> drivers) {

        public FirstOrder {
            drivers = drivers == null ? List.of() : List.copyOf(drivers);
        }

        public List<String> factsConfirmed() {
            return drivers.stream().filter(d -> !d.assumed()).map(Driver::text).toList();
        }

        public List<String> factsAssumed() {
            return drivers.stream().filter(Driver::assumed).map(Driver::text).toList();
        }
    }

    public record Driver(String text, boolean assumed) {
    }
}
