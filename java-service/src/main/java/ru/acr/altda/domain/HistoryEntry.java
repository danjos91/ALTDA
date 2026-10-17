package ru.acr.altda.domain;

import java.time.Instant;

/**
 * Approval / revision event recorded for a program (methodologist feedback loop).
 */
public record HistoryEntry(
        Instant at,
        String action,
        ProgramStatus fromStatus,
        ProgramStatus toStatus,
        String comment,
        String variantKey
) {
}
