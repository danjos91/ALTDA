package ru.acr.altda.api.dto;

import jakarta.validation.constraints.NotEmpty;

import java.util.List;

public record LockNodesRequest(
        @NotEmpty List<String> nodeIds,
        boolean locked
) {
}
