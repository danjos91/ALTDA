package ru.acr.altda.go;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import ru.acr.altda.config.GoClientProperties;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * Verifies HttpGoClient maps Go {@code {error,message}} bodies to typed {@link GoClientException}.
 */
class HttpGoClientErrorMappingTest {

    private MockWebServer server;
    private HttpGoClient client;
    private final ObjectMapper mapper = new ObjectMapper();

    @BeforeEach
    void setUp() throws Exception {
        server = new MockWebServer();
        server.start();
        GoClientProperties props = new GoClientProperties();
        props.setBaseUrl(server.url("/").toString().replaceAll("/$", ""));
        props.setConnectTimeoutMs(2_000);
        GoClientProperties.Timeouts timeouts = new GoClientProperties.Timeouts();
        timeouts.setParseMs(5_000);
        timeouts.setSkeletonMs(5_000);
        timeouts.setRegenerateMs(5_000);
        timeouts.setContentMs(5_000);
        timeouts.setValidateMs(5_000);
        timeouts.setAllocateMs(5_000);
        props.setTimeouts(timeouts);
        client = new HttpGoClient(mapper, props);
    }

    @AfterEach
    void tearDown() throws Exception {
        server.shutdown();
    }

    @Test
    void nodeLockedMapsTo409TypedException() {
        enqueueError(409, GoClientException.NODE_LOCKED, "node M1 is locked");
        ObjectNode program = mapper.createObjectNode().put("id", "p1");

        assertThatThrownBy(() -> client.regenerateSkeleton(program, "fix", null, true))
                .isInstanceOf(GoClientException.class)
                .satisfies(ex -> {
                    GoClientException gce = (GoClientException) ex;
                    assertThat(gce.getErrorCode()).isEqualTo(GoClientException.NODE_LOCKED);
                    assertThat(gce.isNodeLocked()).isTrue();
                    assertThat(gce.canonicalHttpStatus()).isEqualTo(409);
                    assertThat(gce.shouldMarkProgramFailed(false)).isFalse();
                    assertThat(gce.shouldMarkProgramFailed(true)).isFalse();
                    assertThat(gce.getMessage()).contains("locked");
                });
    }

    @Test
    void badRequestMapsTo400_doesNotFailProgramOnSync() {
        enqueueError(400, GoClientException.BAD_REQUEST, "missing rawText");

        assertThatThrownBy(() -> client.parse("", mapper.createArrayNode()))
                .isInstanceOf(GoClientException.class)
                .satisfies(ex -> {
                    GoClientException gce = (GoClientException) ex;
                    assertThat(gce.getErrorCode()).isEqualTo(GoClientException.BAD_REQUEST);
                    assertThat(gce.isClientError()).isTrue();
                    assertThat(gce.canonicalHttpStatus()).isEqualTo(400);
                    assertThat(gce.shouldMarkProgramFailed(false)).isFalse();
                    assertThat(gce.shouldMarkProgramFailed(true)).isTrue();
                });
    }

    @Test
    void schemaInvalidMapsTo422() {
        enqueueError(422, GoClientException.SCHEMA_INVALID, "program failed schema");
        ObjectNode program = mapper.createObjectNode().put("id", "p1");

        assertThatThrownBy(() -> client.validate(program))
                .isInstanceOf(GoClientException.class)
                .satisfies(ex -> {
                    GoClientException gce = (GoClientException) ex;
                    assertThat(gce.getErrorCode()).isEqualTo(GoClientException.SCHEMA_INVALID);
                    assertThat(gce.canonicalHttpStatus()).isEqualTo(422);
                    assertThat(gce.shouldMarkProgramFailed(false)).isFalse();
                    assertThat(gce.shouldMarkProgramFailed(true)).isTrue();
                });
    }

    @Test
    void llmTimeoutMapsTo504Only_never408() {
        // Even if Go answered with 408, Java maps llm_timeout → canonical 504 only
        enqueueError(408, GoClientException.LLM_TIMEOUT, "model timed out");
        ObjectNode program = mapper.createObjectNode().put("id", "p1");

        assertThatThrownBy(() -> client.generateContent(program, null))
                .isInstanceOf(GoClientException.class)
                .satisfies(ex -> {
                    GoClientException gce = (GoClientException) ex;
                    assertThat(gce.getErrorCode()).isEqualTo(GoClientException.LLM_TIMEOUT);
                    assertThat(gce.isLlmOrTransportFailure()).isTrue();
                    assertThat(gce.canonicalHttpStatus()).isEqualTo(504);
                    assertThat(gce.getHttpStatus()).isEqualTo(504);
                    assertThat(gce.shouldMarkProgramFailed(false)).isTrue();
                    assertThat(gce.shouldMarkProgramFailed(true)).isTrue();
                });
    }

    @Test
    void timeoutDetectedViaHttpTimeoutExceptionCause() {
        RuntimeException wrapped = new RuntimeException(
                "call failed",
                new java.net.http.HttpTimeoutException("request timed out"));
        assertThat(HttpGoClient.isTimeout(wrapped)).isTrue();
        assertThat(HttpGoClient.isTimeout(new RuntimeException("other"))).isFalse();
    }

    @Test
    void contentIssuesNullCoalescesToEmptyArray() throws Exception {
        server.enqueue(new MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody("{\"schemaVersion\":\"1.1\",\"program\":{\"id\":\"p1\",\"schemaVersion\":\"1.1\"},\"issues\":null}"));
        ObjectNode program = mapper.createObjectNode().put("id", "p1");
        GoClient.ContentGenerateResult result = client.generateContent(program, null);
        assertThat(result.issues().isArray()).isTrue();
        assertThat(result.issues()).isEmpty();
    }

    @Test
    void llmErrorMapsTo502_marksFailed() {
        enqueueError(502, GoClientException.LLM_ERROR, "upstream model failure");
        ObjectNode program = mapper.createObjectNode().put("id", "p1");

        assertThatThrownBy(() -> client.generateSkeleton(program))
                .isInstanceOf(GoClientException.class)
                .satisfies(ex -> {
                    GoClientException gce = (GoClientException) ex;
                    assertThat(gce.getErrorCode()).isEqualTo(GoClientException.LLM_ERROR);
                    assertThat(gce.canonicalHttpStatus()).isEqualTo(502);
                    assertThat(gce.shouldMarkProgramFailed(true)).isTrue();
                });
    }

    @Test
    void errorCodeWinsOverHttpStatus_fromGoBody() {
        // Go may send 400 with schema_invalid — Java uses the error code for canonical status
        enqueueError(400, GoClientException.SCHEMA_INVALID, "schema boom");
        ObjectNode program = mapper.createObjectNode().put("id", "p1");

        assertThatThrownBy(() -> client.allocate(program))
                .isInstanceOf(GoClientException.class)
                .satisfies(ex -> {
                    GoClientException gce = (GoClientException) ex;
                    assertThat(gce.getErrorCode()).isEqualTo(GoClientException.SCHEMA_INVALID);
                    assertThat(gce.canonicalHttpStatus()).isEqualTo(422);
                });
    }

    private void enqueueError(int httpStatus, String code, String message) {
        String body = "{\"error\":\"" + code + "\",\"message\":\"" + message + "\"}";
        server.enqueue(new MockResponse()
                .setResponseCode(httpStatus)
                .setHeader("Content-Type", "application/json")
                .setBody(body));
    }
}
