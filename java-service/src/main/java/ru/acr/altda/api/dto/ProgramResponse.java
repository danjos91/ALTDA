package ru.acr.altda.api.dto;

import com.fasterxml.jackson.databind.JsonNode;
import ru.acr.altda.domain.HistoryEntry;

import java.time.Instant;
import java.util.List;
import java.util.UUID;

public record ProgramResponse(
        UUID id,
        String status,
        boolean busy,
        String error,
        String failedFrom,
        long version,
        String selectedVariantKey,
        JsonNode skeletonVariants,
        JsonNode validationReport,
        JsonNode issues,
        JsonNode program,
        List<HistoryEntry> history,
        Instant createdAt,
        Instant updatedAt
) {
}
