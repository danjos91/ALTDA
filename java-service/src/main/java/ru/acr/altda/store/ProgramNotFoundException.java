package ru.acr.altda.store;

import java.util.UUID;

public class ProgramNotFoundException extends RuntimeException {

    private final UUID programId;

    public ProgramNotFoundException(UUID programId) {
        super("Program not found: " + programId);
        this.programId = programId;
    }

    public UUID getProgramId() {
        return programId;
    }
}
