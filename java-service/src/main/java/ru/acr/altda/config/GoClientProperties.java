package ru.acr.altda.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "altda.go")
public class GoClientProperties {

    private String client = "mock";
    private String baseUrl = "http://localhost:8081";
    private long connectTimeoutMs = 5000;
    private Timeouts timeouts = new Timeouts();

    public String getClient() {
        return client;
    }

    public void setClient(String client) {
        this.client = client;
    }

    public String getBaseUrl() {
        return baseUrl;
    }

    public void setBaseUrl(String baseUrl) {
        this.baseUrl = baseUrl;
    }

    public long getConnectTimeoutMs() {
        return connectTimeoutMs;
    }

    public void setConnectTimeoutMs(long connectTimeoutMs) {
        this.connectTimeoutMs = connectTimeoutMs;
    }

    public Timeouts getTimeouts() {
        return timeouts;
    }

    public void setTimeouts(Timeouts timeouts) {
        this.timeouts = timeouts;
    }

    public static class Timeouts {
        private long skeletonMs = 180_000;
        private long regenerateMs = 180_000;
        private long contentMs = 600_000;
        private long validateMs = 10_000;
        private long allocateMs = 10_000;
        private long parseMs = 60_000;

        public long getSkeletonMs() {
            return skeletonMs;
        }

        public void setSkeletonMs(long skeletonMs) {
            this.skeletonMs = skeletonMs;
        }

        public long getRegenerateMs() {
            return regenerateMs;
        }

        public void setRegenerateMs(long regenerateMs) {
            this.regenerateMs = regenerateMs;
        }

        public long getContentMs() {
            return contentMs;
        }

        public void setContentMs(long contentMs) {
            this.contentMs = contentMs;
        }

        public long getValidateMs() {
            return validateMs;
        }

        public void setValidateMs(long validateMs) {
            this.validateMs = validateMs;
        }

        public long getAllocateMs() {
            return allocateMs;
        }

        public void setAllocateMs(long allocateMs) {
            this.allocateMs = allocateMs;
        }

        public long getParseMs() {
            return parseMs;
        }

        public void setParseMs(long parseMs) {
            this.parseMs = parseMs;
        }
    }
}
