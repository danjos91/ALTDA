package ru.acr.altda.api.dto;

public record ContentApproveRequest(
        String comment,
        Long version
) {
}
