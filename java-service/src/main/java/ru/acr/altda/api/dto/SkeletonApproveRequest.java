package ru.acr.altda.api.dto;

import jakarta.validation.constraints.NotBlank;

public record SkeletonApproveRequest(
        @NotBlank String variantKey,
        String comment,
        Long version
) {
}
