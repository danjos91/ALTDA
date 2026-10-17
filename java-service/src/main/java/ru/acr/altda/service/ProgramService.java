package ru.acr.altda.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.springframework.core.task.TaskRejectedException;
import org.springframework.stereotype.Service;
import ru.acr.altda.api.dto.ClarificationAnswerRequest;
import ru.acr.altda.api.dto.ContentApproveRequest;
import ru.acr.altda.api.dto.CreateProgramRequest;
import ru.acr.altda.api.dto.GenerateRequest;
import ru.acr.altda.api.dto.GenerateResponse;
import ru.acr.altda.api.dto.LockNodesRequest;
import ru.acr.altda.api.dto.RegenerateRequest;
import ru.acr.altda.api.dto.ReturnRequest;
import ru.acr.altda.api.dto.SkeletonApproveRequest;
import ru.acr.altda.domain.HistoryEntry;
import ru.acr.altda.domain.PipelineStep;
import ru.acr.altda.domain.ProgramStatus;
import ru.acr.altda.domain.ProgramStatusMachine;
import ru.acr.altda.go.GoClient;
import ru.acr.altda.go.GoClientException;
import ru.acr.altda.store.ProgramStore;

import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.UUID;

@Service
public class ProgramService {

    private final ProgramStore store;
    private final GoClient goClient;
    private final ObjectMapper objectMapper;
    private final ProgramAsyncJobs asyncJobs;
    private final SkeletonMaterializer skeletonMaterializer;
    private final ProgramServiceSupport support;
    private final ProgramJsonHelpers jsonHelpers;

    public ProgramService(
            ProgramStore store,
            GoClient goClient,
            ObjectMapper objectMapper,
            ProgramAsyncJobs asyncJobs,
            SkeletonMaterializer skeletonMaterializer,
            ProgramServiceSupport support,
            ProgramJsonHelpers jsonHelpers
    ) {
        this.store = store;
        this.goClient = goClient;
        this.objectMapper = objectMapper;
        this.asyncJobs = asyncJobs;
        this.skeletonMaterializer = skeletonMaterializer;
        this.support = support;
        this.jsonHelpers = jsonHelpers;
    }

    public ProgramStore.ProgramSnapshot create(CreateProgramRequest request) {
        UUID id = UUID.randomUUID();
        ObjectNode program = objectMapper.createObjectNode();
        program.put("schemaVersion", "1.1");
        program.put("id", id.toString());
        program.put("status", ProgramStatus.INTAKE.jsonValue());
        ObjectNode req = program.putObject("request");
        req.put("rawText", request.rawText());
        req.putArray("attachments");
        req.putArray("missing");
        req.putArray("clarifications");
        req.putArray("gapsReport");

        store.saveNew(new ru.acr.altda.domain.ProgramRecord(id, program));
        dispatchAsync(id, PipelineStep.PARSE, () -> asyncJobs.runParseAndMaybeSkeleton(id));
        return store.snapshot(id);
    }

    public List<ProgramStore.ProgramSnapshot> list() {
        return store.snapshotAll();
    }

    public ProgramStore.ProgramSnapshot get(UUID id) {
        return store.snapshot(id);
    }

    public ProgramStore.ProgramSnapshot answerClarifications(
            UUID id,
            ClarificationAnswerRequest body,
            Long expectedVersion
    ) {
        store.updateEditable(id, expectedVersion, r -> {
            if (r.getStatus() != ProgramStatus.CLARIFYING && r.getStatus() != ProgramStatus.FAILED) {
                throw new IllegalStateException(
                        "Clarifications only accepted in clarifying/failed, was " + r.getStatus().jsonValue());
            }
            ObjectNode request = r.getProgram().withObject("/request");
            ArrayNode clarifications = request.withArray("clarifications");
            for (ClarificationAnswerRequest.Answer answer : body.answers()) {
                boolean found = false;
                for (JsonNode node : clarifications) {
                    if (answer.param().equals(node.path("param").asText())) {
                        ((ObjectNode) node).put("answer", answer.answer());
                        found = true;
                        break;
                    }
                }
                if (!found) {
                    ObjectNode added = clarifications.addObject();
                    added.put("param", answer.param());
                    added.put("question", "");
                    added.put("answer", answer.answer());
                }
            }
            r.clearError();
            r.setBusy(true);
            r.addHistory(new HistoryEntry(
                    Instant.now(), "clarifications_submitted", r.getStatus(), r.getStatus(), null, null));
        });
        retryOr(id, PipelineStep.PARSE);
        return store.snapshot(id);
    }

    public ProgramStore.ProgramSnapshot approveSkeleton(
            UUID id,
            SkeletonApproveRequest body,
            Long expectedVersion
    ) {
        store.updateEditable(id, expectedVersion, r -> {
            if (r.getStatus() != ProgramStatus.SKELETON_PROPOSED) {
                throw new IllegalStateException(
                        "Skeleton approve requires skeleton_proposed, was " + r.getStatus().jsonValue());
            }
            if (r.getSkeletonVariants() == null) {
                throw new IllegalStateException("No skeleton variants available");
            }
            JsonNode chosen = ProgramJsonHelpers.findVariant(r.getSkeletonVariants(), body.variantKey());
            if (chosen == null) {
                throw new IllegalArgumentException("Unknown variantKey: " + body.variantKey());
            }
            ProgramStatus from = r.getStatus();
            ProgramStatusMachine.assertTransition(from, ProgramStatus.SKELETON_APPROVED);
            ObjectNode materialized = skeletonMaterializer.mergeVariant(r.getProgram().deepCopy(), chosen);
            materialized.put("status", ProgramStatus.SKELETON_APPROVED.jsonValue());
            r.setProgram(materialized);
            r.setSelectedVariantKey(body.variantKey());
            r.clearError();
            r.setStatus(ProgramStatus.SKELETON_APPROVED);
            appendApproval(r.getProgram(), "skeleton", true, body.comment());
            r.addHistory(new HistoryEntry(
                    Instant.now(), "skeleton_approved", from, ProgramStatus.SKELETON_APPROVED,
                    body.comment(), body.variantKey()));
            r.setBusy(true);
        });
        dispatchAsync(id, PipelineStep.CONTENT, () -> asyncJobs.runContentAndValidate(id));
        return store.snapshot(id);
    }

    public ProgramStore.ProgramSnapshot returnSkeleton(UUID id, ReturnRequest body, Long expectedVersion) {
        store.updateEditable(id, expectedVersion, r -> {
            if (r.getStatus() != ProgramStatus.SKELETON_PROPOSED && r.getStatus() != ProgramStatus.FAILED) {
                throw new IllegalStateException(
                        "Skeleton return requires skeleton_proposed/failed, was " + r.getStatus().jsonValue());
            }
            ProgramStatus from = r.getStatus();
            if (from != ProgramStatus.FAILED) {
                ProgramStatusMachine.assertTransition(from, ProgramStatus.SKELETON_PROPOSED);
            }
            appendApproval(r.getProgram(), "skeleton", false, body.comment());
            r.clearError();
            r.setStatus(ProgramStatus.SKELETON_PROPOSED);
            r.setBusy(true);
            r.addHistory(new HistoryEntry(
                    Instant.now(), "skeleton_returned", from, ProgramStatus.SKELETON_PROPOSED,
                    body.comment(), null));
        });
        String comment = body.comment();
        retryOr(id, PipelineStep.REGENERATE, () -> asyncJobs.runRegenerateSkeleton(id, comment));
        return store.snapshot(id);
    }

    public ProgramStore.ProgramSnapshot approveContent(
            UUID id,
            ContentApproveRequest body,
            Long expectedVersion
    ) {
        store.updateEditable(id, expectedVersion, r -> {
            if (r.getStatus() != ProgramStatus.VALIDATED && r.getStatus() != ProgramStatus.CONTENT_READY) {
                throw new IllegalStateException(
                        "Content approve requires validated/content_ready, was " + r.getStatus().jsonValue());
            }
            ProgramStatus from = r.getStatus();
            appendApproval(r.getProgram(), "content", true, body == null ? null : body.comment());
            if (from == ProgramStatus.CONTENT_READY) {
                ProgramStatusMachine.assertTransition(from, ProgramStatus.VALIDATED);
                r.setStatus(ProgramStatus.VALIDATED);
            }
            r.addHistory(new HistoryEntry(
                    Instant.now(), "content_approved", from, r.getStatus(),
                    body == null ? null : body.comment(), null));
        });
        return store.snapshot(id);
    }

    public ProgramStore.ProgramSnapshot returnContent(UUID id, ReturnRequest body, Long expectedVersion) {
        store.updateEditable(id, expectedVersion, r -> {
            if (r.getStatus() != ProgramStatus.VALIDATED
                    && r.getStatus() != ProgramStatus.CONTENT_READY
                    && r.getStatus() != ProgramStatus.FAILED) {
                throw new IllegalStateException(
                        "Content return not allowed from " + r.getStatus().jsonValue());
            }
            ProgramStatus from = r.getStatus();
            if (from != ProgramStatus.FAILED) {
                ProgramStatusMachine.assertTransition(from, ProgramStatus.CONTENT_GENERATING);
            }
            appendApproval(r.getProgram(), "content", false, body.comment());
            r.clearError();
            r.setStatus(ProgramStatus.CONTENT_GENERATING);
            r.setBusy(true);
            r.addHistory(new HistoryEntry(
                    Instant.now(), "content_returned", from, ProgramStatus.CONTENT_GENERATING,
                    body.comment(), null));
        });
        retryOr(id, PipelineStep.CONTENT);
        return store.snapshot(id);
    }

    public ProgramStore.ProgramSnapshot regenerate(UUID id, RegenerateRequest body, Long expectedVersion) {
        String scope = body.scope() == null ? "content" : body.scope();
        String comment = body.comment() == null ? "точечная перегенерация" : body.comment();

        if ("skeleton".equals(scope)) {
            ArrayNode nodeIds = toArrayNode(body.nodeIds());
            ObjectNode programCopy = store.programCopy(id);
            try {
                GoClient.SkeletonRegenerateResult result =
                        goClient.regenerateSkeleton(programCopy, comment, nodeIds, true);
                store.updateEditable(id, expectedVersion, r -> {
                    ProgramStatus from = r.getStatus();
                    if (result.variants() != null && !result.variants().isNull()) {
                        r.setSkeletonVariants(jsonHelpers.wrapVariants(result.variants()));
                    }
                    if (result.program() != null) {
                        ObjectNode prog = result.program();
                        prog.put("id", id.toString());
                        r.setProgram(prog);
                    }
                    ProgramStatusMachine.assertTransition(from, ProgramStatus.SKELETON_PROPOSED);
                    r.setStatus(ProgramStatus.SKELETON_PROPOSED);
                    r.setBusy(false);
                    r.clearError();
                    appendApproval(r.getProgram(), "skeleton", false, comment);
                    r.addHistory(new HistoryEntry(
                            Instant.now(), "skeleton_regenerated", from, ProgramStatus.SKELETON_PROPOSED,
                            comment, null));
                });
                return store.snapshot(id);
            } catch (GoClientException ex) {
                // Sync path: surface typed Go error to UI; never mark FAILED here.
                // node_locked → 409, bad_request → 400, schema_invalid → 422 via ApiExceptionHandler.
                throw ex;
            }
        }

        store.updateEditable(id, expectedVersion, r -> {
            ProgramStatus from = r.getStatus();
            if (from != ProgramStatus.CONTENT_GENERATING) {
                ProgramStatusMachine.assertTransition(from, ProgramStatus.CONTENT_GENERATING);
                r.setStatus(ProgramStatus.CONTENT_GENERATING);
            }
            r.clearError();
            r.setBusy(true);
            r.addHistory(new HistoryEntry(
                    Instant.now(), "content_regenerate", from, ProgramStatus.CONTENT_GENERATING,
                    comment, null));
        });
        dispatchAsync(id, PipelineStep.CONTENT, () -> asyncJobs.runContentAndValidate(id));
        return store.snapshot(id);
    }

    public ProgramStore.ProgramSnapshot allocate(UUID id, Long expectedVersion) {
        ObjectNode copy = store.programCopy(id);
        GoClient.AllocateResult result = goClient.allocate(copy);
        store.updateEditable(id, expectedVersion, r -> {
            ObjectNode prog = result.program();
            prog.put("id", id.toString());
            r.setProgram(prog);
            r.addHistory(new HistoryEntry(
                    Instant.now(), "hours_reallocated", r.getStatus(), r.getStatus(),
                    "manual edit rebalance", null));
        });
        return store.snapshot(id);
    }

    public ProgramStore.ProgramSnapshot lockNodes(UUID id, LockNodesRequest body, Long expectedVersion) {
        store.updateEditable(id, expectedVersion, r -> {
            ObjectNode program = r.getProgram();
            for (String nodeId : body.nodeIds()) {
                applyLock(program, nodeId, body.locked());
            }
            r.setProgram(program);
            r.addHistory(new HistoryEntry(
                    Instant.now(),
                    body.locked() ? "nodes_locked" : "nodes_unlocked",
                    r.getStatus(),
                    r.getStatus(),
                    String.join(",", body.nodeIds()),
                    null));
        });
        return store.snapshot(id);
    }

    /**
     * Synchronous test-mode pipeline. State updates run under the store lock;
     * Go calls use deep copies of the program JSON.
     */
    public GenerateResponse generateSync(GenerateRequest request) {
        UUID id = UUID.randomUUID();
        ObjectNode program = objectMapper.createObjectNode();
        program.put("schemaVersion", "1.1");
        program.put("id", id.toString());
        program.put("status", ProgramStatus.INTAKE.jsonValue());
        ObjectNode reqNode = program.putObject("request");
        reqNode.put("rawText", request.request());
        reqNode.putArray("attachments");
        ArrayNode clarifications = reqNode.putArray("clarifications");
        if (request.answers() != null) {
            for (Map.Entry<String, String> entry : request.answers().entrySet()) {
                ObjectNode c = clarifications.addObject();
                c.put("param", entry.getKey());
                c.put("question", "");
                c.put("answer", entry.getValue());
            }
        }
        reqNode.putArray("missing");
        reqNode.putArray("gapsReport");
        store.saveNew(new ru.acr.altda.domain.ProgramRecord(id, program));

        try {
            ObjectNode copy = store.programCopy(id);
            GoClient.ParseResult firstParse = goClient.parse(
                    request.request(),
                    copy.path("request").path("clarifications").deepCopy());
            store.update(id, r -> support.applyParseResult(r, firstParse));

            if (firstParse.missing() != null && firstParse.missing().isArray() && !firstParse.missing().isEmpty()) {
                store.update(id, r -> {
                    ObjectNode requestObj = r.getProgram().withObject("/request");
                    ArrayNode existing = requestObj.withArray("clarifications");
                    for (JsonNode missing : firstParse.missing()) {
                        ObjectNode a = existing.addObject();
                        a.put("param", missing.asText());
                        a.put("question", "auto");
                        a.put("answer", "авто-значение для теста");
                    }
                });
                ObjectNode again = store.programCopy(id);
                GoClient.ParseResult secondParse = goClient.parse(
                        request.request(), again.path("request").path("clarifications").deepCopy());
                store.update(id, r -> support.applyParseResult(r, secondParse));
            }

            ObjectNode forSkeleton = store.programCopy(id);
            GoClient.SkeletonGenerateResult skeleton = goClient.generateSkeleton(forSkeleton);
            JsonNode firstVariant = ProgramJsonHelpers.firstVariant(skeleton.variants());
            if (firstVariant == null) {
                throw new IllegalStateException("Go returned no skeleton variants");
            }
            String variantKey = firstVariant.path("key").asText("A");
            store.update(id, r -> {
                r.setSkeletonVariants(jsonHelpers.wrapVariants(skeleton.variants()));
                ObjectNode materialized = skeletonMaterializer.mergeVariant(r.getProgram().deepCopy(), firstVariant);
                materialized.put("status", ProgramStatus.SKELETON_APPROVED.jsonValue());
                r.setProgram(materialized);
                r.setSelectedVariantKey(variantKey);
                r.setStatus(ProgramStatus.SKELETON_APPROVED);
                appendApproval(r.getProgram(), "skeleton", true, "auto-approve (test mode)");
                r.addHistory(new HistoryEntry(
                        Instant.now(), "skeleton_auto_approved", ProgramStatus.SKELETON_PROPOSED,
                        ProgramStatus.SKELETON_APPROVED, "test mode", variantKey));
                r.setStatus(ProgramStatus.CONTENT_GENERATING);
            });

            ObjectNode forContent = store.programCopy(id);
            GoClient.ContentGenerateResult content = goClient.generateContent(forContent, null);
            store.update(id, r -> {
                ObjectNode contentProgram = content.program();
                contentProgram.put("id", id.toString());
                contentProgram.put("status", ProgramStatus.CONTENT_READY.jsonValue());
                r.setProgram(contentProgram);
                r.setStatus(ProgramStatus.CONTENT_READY);
                if (content.issues() != null) {
                    r.setIssues(content.issues());
                }
                appendApproval(r.getProgram(), "content", true, "auto-approve (test mode)");
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
                r.setStatus(ProgramStatus.VALIDATED);
                r.setBusy(false);
                r.clearError();
            });

            ProgramStore.ProgramSnapshot done = store.snapshot(id);
            return new GenerateResponse(
                    id,
                    done.status().jsonValue(),
                    done.program(),
                    done.issues(),
                    done.validationReport()
            );
        } catch (GoClientException ex) {
            if (ex.shouldMarkProgramFailed(false)) {
                // Sync test mode: only LLM/transport failures mark FAILED
                store.update(id, r -> r.markFailed(ex.getMessage(), PipelineStep.CONTENT));
            }
            throw ex;
        } catch (Exception ex) {
            store.update(id, r -> r.markFailed(ex.getMessage(), PipelineStep.CONTENT));
            throw ex;
        }
    }

    private void retryOr(UUID id, PipelineStep defaultStep) {
        retryOr(id, defaultStep, null);
    }

    private void retryOr(UUID id, PipelineStep defaultStep, Runnable regenerateAction) {
        ProgramStore.ProgramSnapshot snap = store.snapshot(id);
        PipelineStep step = snap.failedFrom() != null ? snap.failedFrom() : defaultStep;
        switch (step) {
            case PARSE -> dispatchAsync(id, PipelineStep.PARSE, () -> asyncJobs.runParseAndMaybeSkeleton(id));
            case SKELETON -> dispatchAsync(id, PipelineStep.SKELETON, () -> asyncJobs.runSkeletonOnly(id));
            case REGENERATE -> {
                if (regenerateAction != null) {
                    dispatchAsync(id, PipelineStep.REGENERATE, regenerateAction);
                } else {
                    dispatchAsync(id, PipelineStep.REGENERATE,
                            () -> asyncJobs.runRegenerateSkeleton(id, "retry"));
                }
            }
            case CONTENT -> dispatchAsync(id, PipelineStep.CONTENT, () -> asyncJobs.runContentAndValidate(id));
            case VALIDATE -> dispatchAsync(id, PipelineStep.VALIDATE, () -> asyncJobs.runValidateOnly(id));
            default -> throw new IllegalStateException("Unhandled pipeline step: " + step);
        }
    }

    private void dispatchAsync(UUID id, PipelineStep step, Runnable asyncCall) {
        try {
            asyncCall.run();
        } catch (TaskRejectedException ex) {
            store.update(id, r -> r.markFailed("Async executor rejected task: " + ex.getMessage(), step));
        }
    }

    private ArrayNode toArrayNode(List<String> ids) {
        ArrayNode arr = objectMapper.createArrayNode();
        if (ids != null) {
            ids.forEach(arr::add);
        }
        return arr;
    }

    private void applyLock(JsonNode node, String nodeId, boolean locked) {
        if (node == null) {
            return;
        }
        if (node.isObject()) {
            ObjectNode obj = (ObjectNode) node;
            if (nodeId.equals(obj.path("id").asText(null))) {
                ObjectNode meta = obj.withObject("/_meta");
                meta.put("locked", locked);
            }
            obj.fields().forEachRemaining(e -> applyLock(e.getValue(), nodeId, locked));
        } else if (node.isArray()) {
            for (JsonNode child : node) {
                applyLock(child, nodeId, locked);
            }
        }
    }

    private void appendApproval(ObjectNode program, String stage, boolean approved, String comment) {
        ArrayNode approvals = program.withArray("approvals");
        ObjectNode entry = approvals.addObject();
        entry.put("stage", stage);
        entry.put("approved", approved);
        entry.put("at", Instant.now().toString());
        if (comment != null) {
            entry.put("comment", comment);
        }
    }
}
