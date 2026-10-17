package ru.acr.altda.store;

import java.util.UUID;

public class OptimisticLockException extends RuntimeException {

    private final UUID programId;
    private final long expectedVersion;
    private final long actualVersion;

    public OptimisticLockException(UUID programId, long expectedVersion, long actualVersion) {
        super("Optimistic lock failed for " + programId
                + ": expected version " + expectedVersion + ", actual " + actualVersion);
        this.programId = programId;
        this.expectedVersion = expectedVersion;
        this.actualVersion = actualVersion;
    }

    public UUID getProgramId() {
        return programId;
    }

    public long getExpectedVersion() {
        return expectedVersion;
    }

    public long getActualVersion() {
        return actualVersion;
    }
}
