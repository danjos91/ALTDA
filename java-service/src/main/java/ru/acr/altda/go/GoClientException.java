package ru.acr.altda.go;

/**
 * Typed failure from the Go skills service. {@link #getErrorCode()} is one of the fixed contract
 * codes when the Go body provided it: {@code bad_request}, {@code node_locked},
 * {@code llm_timeout}, {@code llm_error}, {@code schema_invalid}.
 *
 * <p>Policy for program state:
 * <ul>
 *   <li>{@code node_locked} — never marks the program {@code FAILED}; sync callers surface HTTP 409.</li>
 *   <li>{@code bad_request} / {@code schema_invalid} — sync: 4xx/422 to the UI, program unchanged;
 *       async pipeline steps: may mark {@code FAILED} (cannot return HTTP mid-job).</li>
 *   <li>{@code llm_timeout} (HTTP <b>504 only</b>) / {@code llm_error} (502), including
 *       connection/read timeouts mapped to those codes — mark the program {@code FAILED}.</li>
 * </ul>
 */
public class GoClientException extends RuntimeException {

    public static final String BAD_REQUEST = "bad_request";
    public static final String NODE_LOCKED = "node_locked";
    public static final String LLM_TIMEOUT = "llm_timeout";
    public static final String LLM_ERROR = "llm_error";
    public static final String SCHEMA_INVALID = "schema_invalid";

    private final String errorCode;
    private final int httpStatus;

    public GoClientException(String errorCode, String message, int httpStatus) {
        super(message);
        this.errorCode = normalize(errorCode);
        this.httpStatus = httpStatus > 0 ? httpStatus : canonicalHttpStatus(this.errorCode);
    }

    public GoClientException(String errorCode, String message, int httpStatus, Throwable cause) {
        super(message, cause);
        this.errorCode = normalize(errorCode);
        this.httpStatus = httpStatus > 0 ? httpStatus : canonicalHttpStatus(this.errorCode);
    }

    public static GoClientException of(String errorCode, String message) {
        String code = normalize(errorCode);
        return new GoClientException(code, message, canonicalHttpStatus(code));
    }

    public String getErrorCode() {
        return errorCode;
    }

    public int getHttpStatus() {
        return httpStatus;
    }

    public boolean isNodeLocked() {
        return NODE_LOCKED.equals(errorCode);
    }

    public boolean isClientError() {
        return BAD_REQUEST.equals(errorCode) || SCHEMA_INVALID.equals(errorCode);
    }

    /** LLM / transport failures that always fail the program. */
    public boolean isLlmOrTransportFailure() {
        return LLM_TIMEOUT.equals(errorCode) || LLM_ERROR.equals(errorCode);
    }

    /**
     * Whether orchestration should transition the program to Java-only {@code FAILED}.
     *
     * @param asyncStep {@code true} when the error occurred inside an async pipeline job
     */
    public boolean shouldMarkProgramFailed(boolean asyncStep) {
        if (isNodeLocked()) {
            return false;
        }
        if (isLlmOrTransportFailure()) {
            return true;
        }
        if (isClientError()) {
            return asyncStep;
        }
        // Unknown codes: fail only inside async (sync path surfaces HTTP instead).
        return asyncStep;
    }

    public int canonicalHttpStatus() {
        return canonicalHttpStatus(errorCode);
    }

    public static int canonicalHttpStatus(String errorCode) {
        return switch (normalize(errorCode)) {
            case NODE_LOCKED -> 409;
            case BAD_REQUEST -> 400;
            case SCHEMA_INVALID -> 422;
            case LLM_TIMEOUT -> 504;
            default -> 502;
        };
    }

    private static String normalize(String errorCode) {
        if (errorCode == null || errorCode.isBlank()) {
            return LLM_ERROR;
        }
        return switch (errorCode.trim()) {
            case BAD_REQUEST, NODE_LOCKED, LLM_TIMEOUT, LLM_ERROR, SCHEMA_INVALID -> errorCode.trim();
            default -> errorCode.trim();
        };
    }
}
