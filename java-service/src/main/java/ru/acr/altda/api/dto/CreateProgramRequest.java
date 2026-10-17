package ru.acr.altda.api.dto;

import jakarta.validation.constraints.NotBlank;

public record CreateProgramRequest(
        @NotBlank String rawText
) {
}
