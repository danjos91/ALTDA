package ru.acr.altda.domain;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;

/**
 * Program lifecycle statuses.
 * Schema statuses match {@code program.schema.json}; {@link #FAILED} is a Java
 * orchestration extension (not written into {@code program.status} JSON).
 */
public enum ProgramStatus {
    INTAKE("intake"),
    CLARIFYING("clarifying"),
    SKELETON_PROPOSED("skeleton_proposed"),
    SKELETON_APPROVED("skeleton_approved"),
    CONTENT_GENERATING("content_generating"),
    CONTENT_READY("content_ready"),
    VALIDATED("validated"),
    EXPORTED("exported"),
    /** Go timeout / unexpected failure — never leave a program stuck mid-async. */
    FAILED("failed");

    private final String jsonValue;

    ProgramStatus(String jsonValue) {
        this.jsonValue = jsonValue;
    }

    @JsonValue
    public String jsonValue() {
        return jsonValue;
    }

    @JsonCreator
    public static ProgramStatus fromJsonValue(String value) {
        return fromJson(value);
    }

    /** Whether this value may appear in {@code program.status} (schema enum). */
    public boolean isSchemaStatus() {
        return this != FAILED;
    }

    public static ProgramStatus fromJson(String value) {
        for (ProgramStatus status : values()) {
            if (status.jsonValue.equals(value)) {
                return status;
            }
        }
        throw new IllegalArgumentException("Unknown program status: " + value);
    }
}
