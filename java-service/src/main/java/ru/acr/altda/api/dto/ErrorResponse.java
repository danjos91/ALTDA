package ru.acr.altda.api.dto;

public record ErrorResponse(
        String error,
        String message
) {
}
