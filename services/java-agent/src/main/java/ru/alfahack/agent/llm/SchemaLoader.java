package ru.alfahack.agent.llm;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.util.concurrent.ConcurrentHashMap;
import java.util.regex.Pattern;

import org.springframework.stereotype.Component;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import lombok.RequiredArgsConstructor;

@Component
@RequiredArgsConstructor
public class SchemaLoader {

    private static final Pattern SAFE_NAME = Pattern.compile("[a-z0-9_]+");

    private final ObjectMapper mapper;
    private final ConcurrentHashMap<String, JsonNode> cache = new ConcurrentHashMap<>();

    public JsonNode load(String name) {
        if (!SAFE_NAME.matcher(name).matches()) {
            throw new IllegalArgumentException("недопустимое имя схемы: " + name);
        }
        return cache.computeIfAbsent(name, this::readSchema);
    }

    private JsonNode readSchema(String name) {
        String path = "/schemas/" + name + ".json";
        try (var in = SchemaLoader.class.getResourceAsStream(path)) {
            if (in == null) {
                throw new IllegalStateException("JSON Schema не найдена в classpath: " + path);
            }
            return mapper.readTree(in);
        } catch (IOException e) {
            throw new UncheckedIOException("не удалось прочитать JSON Schema: " + path, e);
        }
    }
}
