package ru.acr.altda.store;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.springframework.stereotype.Component;
import ru.acr.altda.api.ConflictException;
import ru.acr.altda.domain.HistoryEntry;
import ru.acr.altda.domain.PipelineStep;
import ru.acr.altda.domain.ProgramRecord;
import ru.acr.altda.domain.ProgramStatus;
import ru.acr.altda.domain.ProgramStatusMachine;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Consumer;

/**
 * In-memory store. All mutations run under the record monitor; version bumps
 * exactly once per {@link #update}.
 */
@Component
public class ProgramStore {

    private final ConcurrentHashMap<UUID, ProgramRecord> programs = new ConcurrentHashMap<>();

    public ProgramRecord saveNew(ProgramRecord record) {
        ProgramRecord previous = programs.putIfAbsent(record.getId(), record);
        if (previous != null) {
            throw new IllegalStateException("Program already exists: " + record.getId());
        }
        synchronized (record) {
            record.bumpVersionOnce();
        }
        return record;
    }

    public Optional<ProgramRecord> find(UUID id) {
        return Optional.ofNullable(programs.get(id));
    }

    public ProgramRecord require(UUID id) {
        ProgramRecord record = programs.get(id);
        if (record == null) {
            throw new ProgramNotFoundException(id);
        }
        return record;
    }

    public List<ProgramRecord> findAll() {
        return new ArrayList<>(programs.values());
    }

    /**
     * Deep-copy snapshot taken under the record lock (safe for GET responses / Go calls).
     */
    public ProgramSnapshot snapshot(UUID id) {
        ProgramRecord record = require(id);
        synchronized (record) {
            return ProgramSnapshot.from(record);
        }
    }

    public List<ProgramSnapshot> snapshotAll() {
        List<ProgramSnapshot> out = new ArrayList<>();
        for (ProgramRecord record : programs.values()) {
            synchronized (record) {
                out.add(ProgramSnapshot.from(record));
            }
        }
        return out;
    }

    /**
     * Mutate under the record lock. Optionally enforce optimistic {@code expectedVersion}.
     * Asserts editability (busy/status) inside the lock before the mutator runs.
     * Version increments exactly once after a successful mutator.
     */
    public ProgramRecord update(UUID id, Long expectedVersion, boolean requireEditable, Consumer<ProgramRecord> mutator) {
        ProgramRecord record = require(id);
        synchronized (record) {
            if (expectedVersion != null && record.getVersion() != expectedVersion) {
                throw new OptimisticLockException(id, expectedVersion, record.getVersion());
            }
            if (requireEditable) {
                assertEditableLocked(record);
            }
            mutator.accept(record);
            record.bumpVersionOnce();
            return record;
        }
    }

    public ProgramRecord update(UUID id, Consumer<ProgramRecord> mutator) {
        return update(id, null, false, mutator);
    }

    public ProgramRecord updateEditable(UUID id, Long expectedVersion, Consumer<ProgramRecord> mutator) {
        return update(id, expectedVersion, true, mutator);
    }

    /** Copy of program JSON for Go calls (caller must not mutate the live store node). */
    public ObjectNode programCopy(UUID id) {
        ProgramRecord record = require(id);
        synchronized (record) {
            return record.getProgram().deepCopy();
        }
    }

    private static void assertEditableLocked(ProgramRecord record) {
        if (ProgramStatusMachine.isMutationBlocked(record.getStatus(), record.isBusy())) {
            throw new ConflictException(
                    "Program is busy generating/validating (status="
                            + record.getStatus().jsonValue() + ", busy=" + record.isBusy() + ")");
        }
    }

    /**
     * Immutable view for API responses.
     */
    public record ProgramSnapshot(
            UUID id,
            ProgramStatus status,
            boolean busy,
            String error,
            PipelineStep failedFrom,
            long version,
            String selectedVariantKey,
            JsonNode skeletonVariants,
            JsonNode validationReport,
            JsonNode issues,
            ObjectNode program,
            List<HistoryEntry> history,
            Instant createdAt,
            Instant updatedAt
    ) {
        static ProgramSnapshot from(ProgramRecord r) {
            return new ProgramSnapshot(
                    r.getId(),
                    r.getStatus(),
                    r.isBusy(),
                    r.getError(),
                    r.getFailedFrom(),
                    r.getVersion(),
                    r.getSelectedVariantKey(),
                    r.getSkeletonVariants() == null ? null : r.getSkeletonVariants().deepCopy(),
                    r.getValidationReport() == null ? null : r.getValidationReport().deepCopy(),
                    r.getIssues() == null ? null : r.getIssues().deepCopy(),
                    r.getProgram().deepCopy(),
                    r.getHistory(),
                    r.getCreatedAt(),
                    r.getUpdatedAt()
            );
        }
    }
}
