package ru.acr.altda.domain;

import com.fasterxml.jackson.annotation.JsonValue;

/**
 * Async pipeline step that failed — used to retry only that step.
 */
public enum PipelineStep {
    PARSE("parse"),
    SKELETON("skeleton"),
    REGENERATE("regenerate"),
    CONTENT("content"),
    VALIDATE("validate");

    private final String jsonValue;

    PipelineStep(String jsonValue) {
        this.jsonValue = jsonValue;
    }

    @JsonValue
    public String jsonValue() {
        return jsonValue;
    }
}
