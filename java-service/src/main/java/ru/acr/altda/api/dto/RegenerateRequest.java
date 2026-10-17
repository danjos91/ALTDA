package ru.acr.altda.api.dto;

import java.util.List;

public record RegenerateRequest(
        String comment,
        List<String> nodeIds,
        /** skeleton | content | section */
        String scope
) {
}
