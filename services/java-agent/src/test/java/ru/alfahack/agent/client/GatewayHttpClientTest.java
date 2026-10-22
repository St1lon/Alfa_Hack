package ru.alfahack.agent.client;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.content;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.jsonPath;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.method;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.requestTo;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withServerError;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withStatus;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withSuccess;

import java.time.Duration;
import java.util.List;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.test.web.client.MockRestServiceServer;
import org.springframework.web.client.RestClient;

class GatewayHttpClientTest {

    private static final String URL = "http://gateway.test";

    @Test
    void sendsSnakeCaseRequestAndParsesResponse() {
        var holder = holder();
        holder.server().expect(requestTo(URL + "/v1/complete"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(content().contentType(MediaType.APPLICATION_JSON))
                .andExpect(jsonPath("$.service").value("java-agent"))
                .andExpect(jsonPath("$.task").value("extract"))
                .andExpect(jsonPath("$.max_tokens").value(600))
                .andRespond(withSuccess("""
                        {"model":"qwen3.6-35b-a3b","content":"{}",
                         "usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15},
                         "cost_kop":7,"cache_hit":false,"soft_limit_hit":false,"spent_today_kop":700}
                        """, MediaType.APPLICATION_JSON));

        GatewayCompletion completion = holder.client().complete(request());

        assertThat(completion.model()).isEqualTo("qwen3.6-35b-a3b");
        assertThat(completion.usage().totalTokens()).isEqualTo(15);
        assertThat(completion.costKop()).isEqualTo(7);
        assertThat(completion.cacheHit()).isFalse();
        holder.server().verify();
    }

    @Test
    void retriesOn502AndSucceeds() {
        var holder = holder();
        holder.server().expect(requestTo(URL + "/v1/complete"))
                .andRespond(withServerError());
        holder.server().expect(requestTo(URL + "/v1/complete"))
                .andRespond(withServerError());
        holder.server().expect(requestTo(URL + "/v1/complete"))
                .andRespond(withSuccess("""
                        {"model":"m","content":"ok","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2},
                         "cost_kop":1,"cache_hit":true,"soft_limit_hit":false,"spent_today_kop":1}
                        """, MediaType.APPLICATION_JSON));

        GatewayCompletion completion = holder.client().complete(request());

        assertThat(completion.content()).isEqualTo("ok");
        holder.server().verify();
    }

    @Test
    void budgetExceededIsNotRetried() {
        var holder = holder();
        holder.server().expect(requestTo(URL + "/v1/complete"))
                .andRespond(withStatus(HttpStatus.TOO_MANY_REQUESTS)
                        .contentType(MediaType.APPLICATION_JSON)
                        .body("{\"code\":\"BUDGET_EXCEEDED\",\"description\":\"дневной лимит 500 ₽ исчерпан\"}"));

        assertThatThrownBy(() -> holder.client().complete(request()))
                .isInstanceOf(GatewayException.BudgetExceeded.class)
                .hasMessageContaining("исчерпан");
        holder.server().verify();
    }

    @Test
    void badRequestFailsImmediatelyWithDescription() {
        var holder = holder();
        holder.server().expect(requestTo(URL + "/v1/complete"))
                .andRespond(withStatus(HttpStatus.BAD_REQUEST)
                        .contentType(MediaType.APPLICATION_JSON)
                        .body("{\"code\":\"MODEL_NOT_ALLOWED\",\"description\":\"модель вне allowlist\"}"));

        assertThatThrownBy(() -> holder.client().complete(request()))
                .isInstanceOf(GatewayException.class)
                .hasMessageContaining("модель вне allowlist");
        holder.server().verify();
    }

    @Test
    void upstreamUnavailableAfterAllRetries() {
        var holder = holder();
        for (int i = 0; i < 3; i++) {
            holder.server().expect(requestTo(URL + "/v1/complete"))
                    .andRespond(withServerError());
        }

        assertThatThrownBy(() -> holder.client().complete(request()))
                .isInstanceOf(GatewayException.UpstreamUnavailable.class)
                .hasMessageContaining("после 3 попыток");
        holder.server().verify();
    }

    private static GatewayRequest request() {
        return new GatewayRequest(
                "java-agent", "java", "extract",
                List.of(GatewayRequest.Message.user("проверка")),
                0.1, 600, null, null);
    }

    private static Holder holder() {
        RestClient.Builder builder = RestClient.builder().baseUrl(URL);
        MockRestServiceServer server = MockRestServiceServer.bindTo(builder).build();
        var props = new ru.alfahack.agent.config.AgentProperties(
                new ru.alfahack.agent.config.AgentProperties.Gateway(
                        URL, "", Duration.ofSeconds(1), Duration.ofSeconds(1), 3, true),
                new ru.alfahack.agent.config.AgentProperties.Rag(0.15, 0.5, 60, 3, 20),
                "v0.1", "java", new ru.alfahack.agent.config.AgentProperties.Selfcheck(false));
        return new Holder(
                new GatewayHttpClient(props, ru.alfahack.agent.config.JacksonConfig.createMapper(),
                        builder.build(), Duration.ZERO),
                server);
    }

    private record Holder(GatewayHttpClient client, MockRestServiceServer server) {
    }
}
