package ru.acr.altda.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Async;
import org.springframework.stereotype.Service;
import ru.acr.altda.config.AsyncConfig;
import ru.acr.altda.domain.HistoryEntry;
import ru.acr.altda.domain.PipelineStep;
import ru.acr.altda.domain.ProgramStatus;
import ru.acr.altda.domain.ProgramStatusMachine;
import ru.acr.altda.go.GoClient;
import ru.acr.altda.go.GoClientException;
import ru.acr.altda.store.ProgramStore;

import java.time.Instant;
import java.util.UUID;

@Service
public class ProgramAsyncJobs {

    private static final Logger log = LoggerFactory.getLogger(ProgramAsyncJobs.class);

    private final ProgramStore store;
    private final GoClient goClient;
    private final ProgramServiceSupport support;
    private final ProgramJsonHelpers jsonHelpers;

    public ProgramAsyncJobs(
            ProgramStore store,
            GoClient goClient,
            ProgramServiceSupport support,
            ProgramJsonHelpers jsonHelpers
    ) {
        this.store = store;
        this.goClient = goClient;
        this.support = support;
        this.jsonHelpers = jsonHelpers;
    }

    @Async(AsyncConfig.PROGRAM_EXECUTOR)
    public void runParseAndMaybeSkeleton(UUID id) {
        try {
            store.update(id, r -> r.setBusy(true));
            ObjectNode programCopy = store.programCopy(id);
            String rawText = programCopy.path("request").path("rawText").asText();
            JsonNode clarifications = programCopy.path("request").path("clarifications");

            GoClient.ParseResult parsed = goClient.parse(rawText, clarifications.deepCopy());
            store.update(id, r -> {
                support.applyParseResult(r, parsed);
                boolean missing = parsed.missing() != null
                        && parsed.missing().isArray()
                        && !parsed.missing().isEmpty();
                ProgramStatus from = r.getStatus();
                if (missing) {
                    ProgramStatusMachine.assertTransition(from, ProgramStatus.CLARIFYING);
                    r.setStatus(ProgramStatus.CLARIFYING);
                    r.setBusy(false);
                    r.clearError();
                    r.addHistory(new HistoryEntry(
                            Instant.now(), "clarifying", from, ProgramStatus.CLARIFYING, null, null));
                } else {
                    r.setBusy(true);
                }
            });

            if (store.snapshot(id).status() == ProgramStatus.CLARIFYING) {
                return;
            }

            generateSkeletonInternal(id);
        } catch (Exception ex) {
            log.error("Parse/skeleton failed for {}", id, ex);
            PipelineStep step = ex.getMessage() != null && ex.getMessage().contains("skeleton")
                    ? PipelineStep.SKELETON
                    : PipelineStep.PARSE;
            handleAsyncFailure(id, step, ex);
        }
    }

    @Async(AsyncConfig.PROGRAM_EXECUTOR)
    public void runSkeletonOnly(UUID id) {
        try {
            store.update(id, r -> {
                r.setBusy(true);
                r.clearError();
            });
            generateSkeletonInternal(id);
        } catch (Exception ex) {
            log.error("Skeleton failed for {}", id, ex);
            handleAsyncFailure(id, PipelineStep.SKELETON, ex);
        }
    }

    @Async(AsyncConfig.PROGRAM_EXECUTOR)
    public void runRegenerateSkeleton(UUID id, String comment) {
        try {
            ObjectNode programCopy = store.programCopy(id);
            GoClient.SkeletonRegenerateResult result = goClient.regenerateSkeleton(
                    programCopy, comment, null, true);
            store.update(id, r -> {
                if (result.variants() != null && !result.variants().isNull()) {
                    r.setSkeletonVariants(jsonHelpers.wrapVariants(result.variants()));
                }
                if (result.program() != null) {
                    ObjectNode prog = result.program();
                    prog.put("id", id.toString());
                    r.setProgram(prog);
                }
                ProgramStatus from = r.getStatus();
                ProgramStatusMachine.assertTransition(from, ProgramStatus.SKELETON_PROPOSED);
                r.setStatus(ProgramStatus.SKELETON_PROPOSED);
                r.setBusy(false);
                r.clearError();
                r.addHistory(new HistoryEntry(
                        Instant.now(), "skeleton_regenerated", from, ProgramStatus.SKELETON_PROPOSED,
                        comment, null));
            });
        } catch (Exception ex) {
            log.error("Regenerate skeleton failed for {}", id, ex);
            handleAsyncFailure(id, PipelineStep.REGENERATE, ex);
        }
    }

    @Async(AsyncConfig.PROGRAM_EXECUTOR)
    public void runContentAndValidate(UUID id) {
        try {
            store.update(id, r -> {
                ProgramStatus from = r.getStatus();
                if (from == ProgramStatus.SKELETON_APPROVED || from == ProgramStatus.CONTENT_GENERATING
                        || from == ProgramStatus.FAILED) {
                    if (from == ProgramStatus.SKELETON_APPROVED || from == ProgramStatus.FAILED) {
                        if (from == ProgramStatus.SKELETON_APPROVED) {
                            ProgramStatusMachine.assertTransition(from, ProgramStatus.CONTENT_GENERATING);
                        }
                        r.setStatus(ProgramStatus.CONTENT_GENERATING);
                    }
                }
                r.setBusy(true);
                r.clearError();
            });

            ObjectNode programCopy = store.programCopy(id);
            GoClient.ContentGenerateResult content = goClient.generateContent(programCopy, null);
            store.update(id, r -> {
                ObjectNode prog = content.program();
                prog.put("id", id.toString());
                prog.put("status", ProgramStatus.CONTENT_READY.jsonValue());
                r.setProgram(prog);
                if (content.issues() != null) {
                    r.setIssues(content.issues());
                }
                ProgramStatusMachine.assertTransition(r.getStatus(), ProgramStatus.CONTENT_READY);
                r.setStatus(ProgramStatus.CONTENT_READY);
            });

            ObjectNode forValidate = store.programCopy(id);
            GoClient.ValidateResult validation = goClient.validate(forValidate);
            store.update(id, r -> {
                r.setValidationReport(validation.validationReport());
                r.setIssues(validation.issues());
                ObjectNode prog = r.getProgram();
                prog.set("validationReport", validation.validationReport());
                prog.put("status", ProgramStatus.VALIDATED.jsonValue());
                r.setProgram(prog);
                ProgramStatusMachine.assertTransition(r.getStatus(), ProgramStatus.VALIDATED);
                r.setStatus(ProgramStatus.VALIDATED);
                r.setBusy(false);
                r.clearError();
                r.addHistory(new HistoryEntry(
                        Instant.now(), "validated", ProgramStatus.CONTENT_READY, ProgramStatus.VALIDATED,
                        null, null));
            });
        } catch (Exception ex) {
            log.error("Content/validate failed for {}", id, ex);
            PipelineStep step = store.snapshot(id).status() == ProgramStatus.CONTENT_READY
                    ? PipelineStep.VALIDATE
                    : PipelineStep.CONTENT;
            handleAsyncFailure(id, step, ex);
        }
    }

    @Async(AsyncConfig.PROGRAM_EXECUTOR)
    public void runValidateOnly(UUID id) {
        try {
            store.update(id, r -> r.setBusy(true));
            ObjectNode forValidate = store.programCopy(id);
            GoClient.ValidateResult validation = goClient.validate(forValidate);
            store.update(id, r -> {
                r.setValidationReport(validation.validationReport());
                r.setIssues(validation.issues());
                ObjectNode prog = r.getProgram();
                prog.set("validationReport", validation.validationReport());
                prog.put("status", ProgramStatus.VALIDATED.jsonValue());
                r.setProgram(prog);
                r.setStatus(ProgramStatus.VALIDATED);
                r.setBusy(false);
                r.clearError();
            });
        } catch (Exception ex) {
            log.error("Validate failed for {}", id, ex);
            handleAsyncFailure(id, PipelineStep.VALIDATE, ex);
        }
    }

    /**
     * Async failure policy:
     * <ul>
     *   <li>{@code node_locked} — clear busy only; program body/status unchanged (no FAILED).</li>
     *   <li>{@code bad_request}/{@code schema_invalid} — mark FAILED (cannot surface HTTP mid-job).</li>
     *   <li>{@code llm_timeout}/{@code llm_error}/connection — mark FAILED.</li>
     * </ul>
     */
    private void handleAsyncFailure(UUID id, PipelineStep step, Exception ex) {
        if (ex instanceof GoClientException gce) {
            if (gce.isNodeLocked()) {
                log.warn("node_locked for {} during {} — leaving program unchanged", id, step);
                store.update(id, r -> r.setBusy(false));
                return;
            }
            if (gce.shouldMarkProgramFailed(true)) {
                store.update(id, r -> r.markFailed(gce.getMessage(), step));
                return;
            }
            store.update(id, r -> r.setBusy(false));
            return;
        }
        // Non-Go failures (unexpected) — treat as pipeline failure
        store.update(id, r -> r.markFailed(ex.getMessage(), step));
    }

    private void generateSkeletonInternal(UUID id) {
        ObjectNode programCopy = store.programCopy(id);
        GoClient.SkeletonGenerateResult skeleton = goClient.generateSkeleton(programCopy);
        store.update(id, r -> {
            r.setSkeletonVariants(jsonHelpers.wrapVariants(skeleton.variants()));
            ProgramStatus from = r.getStatus();
            ProgramStatusMachine.assertTransition(from, ProgramStatus.SKELETON_PROPOSED);
            r.setStatus(ProgramStatus.SKELETON_PROPOSED);
            r.setBusy(false);
            r.clearError();
            r.addHistory(new HistoryEntry(
                    Instant.now(), "skeleton_proposed", from, ProgramStatus.SKELETON_PROPOSED, null, null));
        });
    }
}
