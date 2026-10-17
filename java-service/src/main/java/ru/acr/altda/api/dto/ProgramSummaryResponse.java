package ru.acr.altda.api.dto;

import java.time.Instant;
import java.util.UUID;

public record ProgramSummaryResponse(
        UUID id,
        String status,
        String title,
        boolean busy,
        String error,
        long version,
        Instant createdAt,
        Instant updatedAt
) {
}
