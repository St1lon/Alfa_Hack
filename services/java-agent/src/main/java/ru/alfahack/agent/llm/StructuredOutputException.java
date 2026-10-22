package ru.alfahack.agent.llm;

import java.util.List;

public class StructuredOutputException extends RuntimeException {

    private final List<String> errors;

    public StructuredOutputException(String schemaName, List<String> errors) {
        super("ответ модели не прошёл валидацию по схеме %s после авторепейра: %s"
                .formatted(schemaName, errors));
        this.errors = List.copyOf(errors);
    }

    public List<String> errors() {
        return errors;
    }
}
