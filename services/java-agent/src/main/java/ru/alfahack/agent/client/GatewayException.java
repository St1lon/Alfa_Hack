package ru.alfahack.agent.client;

public class GatewayException extends RuntimeException {

    public GatewayException(String message) {
        super(message);
    }

    public GatewayException(String message, Throwable cause) {
        super(message, cause);
    }

    public static final class BudgetExceeded extends GatewayException {

        public BudgetExceeded(String message) {
            super(message);
        }
    }

    public static final class UpstreamUnavailable extends GatewayException {

        public UpstreamUnavailable(String message, Throwable cause) {
            super(message, cause);
        }
    }
}
