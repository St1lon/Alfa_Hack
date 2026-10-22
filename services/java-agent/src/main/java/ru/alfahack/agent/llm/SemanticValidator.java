package ru.alfahack.agent.llm;

import java.util.List;

@FunctionalInterface
public interface SemanticValidator<T> {

    List<String> validate(T value);
}
