package ru.alfahack.agent;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.ConfigurationPropertiesScan;

@SpringBootApplication
@ConfigurationPropertiesScan
public class JavaAgentApplication {

    public static void main(String[] args) {
        SpringApplication.run(JavaAgentApplication.class, args);
    }
}
