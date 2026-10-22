package ru.alfahack.agent.cib;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.util.HashSet;
import java.util.List;
import java.util.Optional;
import java.util.Set;

import org.springframework.stereotype.Component;

import com.fasterxml.jackson.databind.ObjectMapper;

@Component
public class CibCatalog {

    private final ObjectMapper mapper;
    private final List<CibProduct> products;

    public CibCatalog(ObjectMapper mapper) {
        this.mapper = mapper;
        this.products = load();
    }

    public List<CibProduct> all() {
        return products;
    }

    public Optional<CibProduct> byCode(String code) {
        return products.stream().filter(p -> p.code().equals(code)).findFirst();
    }

    private List<CibProduct> load() {
        String path = "/cib/products.json";
        List<CibProduct> loaded;
        try (var in = CibCatalog.class.getResourceAsStream(path)) {
            if (in == null) {
                throw new IllegalStateException("справочник продуктов КИБ не найден: " + path);
            }
            loaded = mapper.readValue(in, mapper.getTypeFactory()
                    .constructCollectionType(List.class, CibProduct.class));
        } catch (IOException e) {
            throw new UncheckedIOException("не удалось прочитать справочник продуктов КИБ", e);
        }

        Set<String> codes = new HashSet<>();
        for (CibProduct product : loaded) {
            if (product.code() == null || product.code().isBlank()) {
                throw new IllegalStateException("продукт без code в справочнике КИБ");
            }
            if (!codes.add(product.code())) {
                throw new IllegalStateException("дублирующийся code продукта КИБ: " + product.code());
            }
            if (product.riskTags() == null || product.riskTags().isEmpty()) {
                throw new IllegalStateException("продукт без тегов риска: " + product.code());
            }
        }
        return List.copyOf(loaded);
    }
}
