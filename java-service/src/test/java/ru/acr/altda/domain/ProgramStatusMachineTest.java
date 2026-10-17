package ru.acr.altda.domain;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ProgramStatusMachineTest {

    @Test
    void happyPathTransitions() {
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.INTAKE, ProgramStatus.CLARIFYING));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.CLARIFYING, ProgramStatus.SKELETON_PROPOSED));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.SKELETON_PROPOSED, ProgramStatus.SKELETON_APPROVED));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.SKELETON_APPROVED, ProgramStatus.CONTENT_GENERATING));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.CONTENT_GENERATING, ProgramStatus.CONTENT_READY));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.CONTENT_READY, ProgramStatus.VALIDATED));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.VALIDATED, ProgramStatus.EXPORTED));
    }

    @Test
    void unlimitedReturnsForCorrections() {
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.SKELETON_PROPOSED, ProgramStatus.SKELETON_PROPOSED));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.CONTENT_READY, ProgramStatus.CONTENT_GENERATING));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.VALIDATED, ProgramStatus.CONTENT_GENERATING));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.VALIDATED, ProgramStatus.SKELETON_PROPOSED));
    }

    @Test
    void failedIsReachableAndRetryable() {
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.CONTENT_GENERATING, ProgramStatus.FAILED));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.FAILED, ProgramStatus.CONTENT_GENERATING));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.FAILED, ProgramStatus.SKELETON_PROPOSED));
    }

    @Test
    void illegalTransitionThrows() {
        assertThrows(IllegalStateException.class,
                () -> ProgramStatusMachine.assertTransition(ProgramStatus.INTAKE, ProgramStatus.EXPORTED));
        assertFalse(ProgramStatusMachine.canTransition(ProgramStatus.EXPORTED, ProgramStatus.INTAKE));
    }

    @Test
    void exportedAllowsReturnToEditing() {
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.EXPORTED, ProgramStatus.VALIDATED));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.EXPORTED, ProgramStatus.CONTENT_GENERATING));
        assertTrue(ProgramStatusMachine.canTransition(ProgramStatus.EXPORTED, ProgramStatus.SKELETON_PROPOSED));
    }

    @Test
    void mutationBlockedWhileBusyOrGenerating() {
        assertTrue(ProgramStatusMachine.isMutationBlocked(ProgramStatus.CLARIFYING, true));
        assertTrue(ProgramStatusMachine.isMutationBlocked(ProgramStatus.CONTENT_GENERATING, false));
        assertFalse(ProgramStatusMachine.isMutationBlocked(ProgramStatus.SKELETON_PROPOSED, false));
    }
}
