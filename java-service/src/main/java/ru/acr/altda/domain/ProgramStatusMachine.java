package ru.acr.altda.domain;

import java.util.EnumSet;
import java.util.Set;

/**
 * Allowed transitions for the заявка lifecycle.
 * Returns for corrections are unlimited. {@link ProgramStatus#FAILED} can retry
 * back into the pipeline.
 */
public final class ProgramStatusMachine {

    private ProgramStatusMachine() {
    }

    public static boolean canTransition(ProgramStatus from, ProgramStatus to) {
        return allowedTargets(from).contains(to);
    }

    public static void assertTransition(ProgramStatus from, ProgramStatus to) {
        if (!canTransition(from, to)) {
            throw new IllegalStateException(
                    "Illegal status transition: " + from.jsonValue() + " -> " + to.jsonValue());
        }
    }

    /**
     * Statuses where mutating API calls must return 409 (async work in flight).
     */
    public static boolean isMutationBlocked(ProgramStatus status, boolean busy) {
        if (busy) {
            return true;
        }
        return status == ProgramStatus.CONTENT_GENERATING;
    }

    public static Set<ProgramStatus> allowedTargets(ProgramStatus from) {
        return switch (from) {
            case INTAKE -> EnumSet.of(
                    ProgramStatus.CLARIFYING,
                    ProgramStatus.SKELETON_PROPOSED,
                    ProgramStatus.FAILED);
            case CLARIFYING -> EnumSet.of(
                    ProgramStatus.CLARIFYING,
                    ProgramStatus.SKELETON_PROPOSED,
                    ProgramStatus.FAILED);
            case SKELETON_PROPOSED -> EnumSet.of(
                    ProgramStatus.SKELETON_APPROVED,
                    ProgramStatus.SKELETON_PROPOSED,
                    ProgramStatus.CLARIFYING,
                    ProgramStatus.FAILED);
            case SKELETON_APPROVED -> EnumSet.of(
                    ProgramStatus.CONTENT_GENERATING,
                    ProgramStatus.FAILED);
            case CONTENT_GENERATING -> EnumSet.of(
                    ProgramStatus.CONTENT_READY,
                    ProgramStatus.FAILED);
            case CONTENT_READY -> EnumSet.of(
                    ProgramStatus.VALIDATED,
                    ProgramStatus.CONTENT_GENERATING,
                    ProgramStatus.FAILED);
            case VALIDATED -> EnumSet.of(
                    ProgramStatus.EXPORTED,
                    ProgramStatus.CONTENT_GENERATING,
                    ProgramStatus.SKELETON_PROPOSED,
                    ProgramStatus.FAILED);
            // Export must not lock the program: allow return to editing after EXPORTED
            case EXPORTED -> EnumSet.of(
                    ProgramStatus.VALIDATED,
                    ProgramStatus.CONTENT_GENERATING,
                    ProgramStatus.SKELETON_PROPOSED,
                    ProgramStatus.FAILED);
            case FAILED -> EnumSet.of(
                    ProgramStatus.CLARIFYING,
                    ProgramStatus.SKELETON_PROPOSED,
                    ProgramStatus.CONTENT_GENERATING,
                    ProgramStatus.INTAKE);
        };
    }
}
