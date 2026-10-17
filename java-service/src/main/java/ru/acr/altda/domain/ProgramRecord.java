package ru.acr.altda.domain;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicLong;

/**
 * In-memory заявка aggregate. Version is bumped exactly once per
 * {@link ru.acr.altda.store.ProgramStore#update} — setters do not increment it.
 */
public class ProgramRecord {

    private final UUID id;
    private final AtomicLong version = new AtomicLong(0);
    private volatile ProgramStatus status;
    private volatile ObjectNode program;
    private volatile JsonNode skeletonVariants;
    private volatile String selectedVariantKey;
    private volatile JsonNode validationReport;
    private volatile JsonNode issues;
    private final List<HistoryEntry> history = new ArrayList<>();
    private volatile boolean busy;
    private volatile String error;
    private volatile PipelineStep failedFrom;
    private final Instant createdAt;
    private volatile Instant updatedAt;

    public ProgramRecord(UUID id, ObjectNode program) {
        this.id = id;
        this.program = program;
        this.status = ProgramStatus.INTAKE;
        this.createdAt = Instant.now();
        this.updatedAt = this.createdAt;
        this.busy = false;
    }

    public UUID getId() {
        return id;
    }

    public long getVersion() {
        return version.get();
    }

    /** Called once by {@code ProgramStore.update} after the mutator returns. */
    public long bumpVersionOnce() {
        long next = version.incrementAndGet();
        this.updatedAt = Instant.now();
        return next;
    }

    public ProgramStatus getStatus() {
        return status;
    }

    public void setStatus(ProgramStatus status) {
        this.status = status;
        if (status != null && status.isSchemaStatus() && program != null) {
            program.put("status", status.jsonValue());
        }
    }

    public ObjectNode getProgram() {
        return program;
    }

    public void setProgram(ObjectNode program) {
        this.program = program;
    }

    public JsonNode getSkeletonVariants() {
        return skeletonVariants;
    }

    public void setSkeletonVariants(JsonNode skeletonVariants) {
        this.skeletonVariants = skeletonVariants;
    }

    public String getSelectedVariantKey() {
        return selectedVariantKey;
    }

    public void setSelectedVariantKey(String selectedVariantKey) {
        this.selectedVariantKey = selectedVariantKey;
    }

    public JsonNode getValidationReport() {
        return validationReport;
    }

    public void setValidationReport(JsonNode validationReport) {
        this.validationReport = validationReport;
    }

    public JsonNode getIssues() {
        return issues;
    }

    public void setIssues(JsonNode issues) {
        this.issues = issues;
    }

    public List<HistoryEntry> getHistory() {
        synchronized (history) {
            return List.copyOf(history);
        }
    }

    public void addHistory(HistoryEntry entry) {
        synchronized (history) {
            history.add(entry);
        }
    }

    public boolean isBusy() {
        return busy;
    }

    public void setBusy(boolean busy) {
        this.busy = busy;
    }

    public String getError() {
        return error;
    }

    public void setError(String error) {
        this.error = error;
    }

    public PipelineStep getFailedFrom() {
        return failedFrom;
    }

    public void setFailedFrom(PipelineStep failedFrom) {
        this.failedFrom = failedFrom;
    }

    public void markFailed(String message, PipelineStep step) {
        this.error = message;
        this.failedFrom = step;
        this.busy = false;
        this.status = ProgramStatus.FAILED;
    }

    public void clearError() {
        this.error = null;
        this.failedFrom = null;
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    public Instant getUpdatedAt() {
        return updatedAt;
    }
}
