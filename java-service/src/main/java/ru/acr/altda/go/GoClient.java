package ru.acr.altda.go;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

/**
 * Sync contract with the Go LLM-skills service. See {@code docs/API.md}.
 * Java never computes hours; Go allocates them (skeleton generate / allocate / content).
 */
public interface GoClient {

    String SCHEMA_VERSION = "1.1";

    /** GET /v1/health */
    HealthResult health();

    ParseResult parse(String rawText, JsonNode clarifications);

    /** Variants already include allocated hours; Java merges the chosen one locally. */
    SkeletonGenerateResult generateSkeleton(ObjectNode program);

    /**
     * With {@code respectLocks=true}, targeting a locked nodeId yields {@link GoClientException}
     * with code {@code node_locked} (HTTP 409).
     */
    SkeletonRegenerateResult regenerateSkeleton(
            ObjectNode program,
            String comment,
            JsonNode nodeIds,
            boolean respectLocks
    );

    /**
     * Rebalance hours after manual edits only.
     * Formulas (every level): {@code contact = lecture+practice+lab},
     * {@code total = contact+selfStudy+assessment}.
     */
    AllocateResult allocate(ObjectNode program);

    /**
     * POST /v1/content — returns {@code program} + {@code issues[]} ({@code issues} never null).
     * Non-null {@code nodeIds}: regenerate only those nodes plus dependents, skipping locked
     * (same targeting semantics as {@link #regenerateSkeleton}).
     */
    ContentGenerateResult generateContent(ObjectNode program, JsonNode nodeIds);

    /** POST /v1/validate — {@code issues} is always an array (never null). */
    ValidateResult validate(ObjectNode program);

    record HealthResult(String status) {
    }

    record ParseResult(
            JsonNode params,
            JsonNode missing,
            JsonNode clarifications,
            JsonNode gapsReport
    ) {
    }

    record SkeletonGenerateResult(JsonNode variants) {
    }

    record SkeletonRegenerateResult(ObjectNode program, JsonNode variants, JsonNode changedPaths) {
    }

    record AllocateResult(ObjectNode program) {
    }

    record ContentGenerateResult(ObjectNode program, JsonNode issues) {
        public ContentGenerateResult {
            if (issues == null || issues.isNull()) {
                throw new IllegalArgumentException("issues must be a non-null array");
            }
            if (!issues.isArray()) {
                throw new IllegalArgumentException("issues must be a JSON array");
            }
        }
    }

    record ValidateResult(JsonNode validationReport, JsonNode issues) {
        public ValidateResult {
            if (issues == null || issues.isNull() || !issues.isArray()) {
                throw new IllegalArgumentException("issues must be a non-null JSON array");
            }
        }
    }
}
