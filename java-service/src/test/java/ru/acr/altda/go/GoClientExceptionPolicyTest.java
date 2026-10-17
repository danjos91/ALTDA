package ru.acr.altda.go;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class GoClientExceptionPolicyTest {

    @Test
    void nodeLockedNeverMarksFailed() {
        GoClientException ex = GoClientException.of(GoClientException.NODE_LOCKED, "locked");
        assertThat(ex.shouldMarkProgramFailed(false)).isFalse();
        assertThat(ex.shouldMarkProgramFailed(true)).isFalse();
        assertThat(ex.canonicalHttpStatus()).isEqualTo(409);
    }

    @Test
    void clientErrorsMarkFailedOnlyInAsync() {
        GoClientException bad = GoClientException.of(GoClientException.BAD_REQUEST, "bad");
        GoClientException schema = GoClientException.of(GoClientException.SCHEMA_INVALID, "schema");
        assertThat(bad.shouldMarkProgramFailed(false)).isFalse();
        assertThat(bad.shouldMarkProgramFailed(true)).isTrue();
        assertThat(schema.shouldMarkProgramFailed(false)).isFalse();
        assertThat(schema.shouldMarkProgramFailed(true)).isTrue();
        assertThat(bad.canonicalHttpStatus()).isEqualTo(400);
        assertThat(schema.canonicalHttpStatus()).isEqualTo(422);
    }

    @Test
    void llmFailuresAlwaysMarkFailed() {
        GoClientException timeout = GoClientException.of(GoClientException.LLM_TIMEOUT, "t");
        GoClientException error = GoClientException.of(GoClientException.LLM_ERROR, "e");
        assertThat(timeout.shouldMarkProgramFailed(false)).isTrue();
        assertThat(timeout.shouldMarkProgramFailed(true)).isTrue();
        assertThat(error.shouldMarkProgramFailed(false)).isTrue();
        assertThat(error.shouldMarkProgramFailed(true)).isTrue();
    }
}
