package ru.acr.altda.api.dto;

import com.fasterxml.jackson.databind.JsonNode;

import java.util.UUID;

public record GenerateResponse(
        UUID id,
        String status,
        JsonNode program,
        JsonNode issues,
        JsonNode validationReport
) {
}
