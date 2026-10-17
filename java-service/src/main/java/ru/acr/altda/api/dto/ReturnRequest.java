package ru.acr.altda.api.dto;

import jakarta.validation.constraints.NotBlank;

public record ReturnRequest(
        @NotBlank String comment,
        Long version
) {
}
