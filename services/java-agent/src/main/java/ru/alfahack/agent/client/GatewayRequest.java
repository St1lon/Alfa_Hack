package ru.alfahack.agent.client;

import java.util.List;
import java.util.Map;

import com.fasterxml.jackson.annotation.JsonInclude;

@JsonInclude(JsonInclude.Include.NON_NULL)
public record GatewayRequest(
        String service,
        String actor,
        String task,
        List<Message> messages,
        Double temperature,
        Integer maxTokens,
        Map<String, Object> responseFormat,
        Boolean cache) {

    public record Message(String role, String content) {

        public static Message system(String content) {
            return new Message("system", content);
        }

        public static Message user(String content) {
            return new Message("user", content);
        }

        public static Message assistant(String content) {
            return new Message("assistant", content);
        }
    }
}
