package ru.acr.altda.api.dto;

import com.fasterxml.jackson.annotation.JsonAlias;
import jakarta.validation.constraints.NotBlank;

import java.util.Map;

/**
 * One-shot test-mode request. Accepts {@code request} (preferred) or {@code rawText}.
 */
public record GenerateRequest(
        @NotBlank
        @JsonAlias({"rawText", "text"})
        String request,
        Map<String, String> answers
) {
}
