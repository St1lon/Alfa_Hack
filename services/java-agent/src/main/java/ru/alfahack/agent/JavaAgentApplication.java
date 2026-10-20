package ru.alfahack.agent;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * Точка входа java-agent. СКЕЛЕТ: заведён Go #1, наполняет Java-разработчик.
 */
@SpringBootApplication
public class JavaAgentApplication {

    public static void main(String[] args) {
        SpringApplication.run(JavaAgentApplication.class, args);
    }
}
