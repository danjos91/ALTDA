package ru.acr.altda.api;

import jakarta.validation.Valid;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import ru.acr.altda.api.dto.ClarificationAnswerRequest;
import ru.acr.altda.api.dto.ContentApproveRequest;
import ru.acr.altda.api.dto.CreateProgramRequest;
import ru.acr.altda.api.dto.LockNodesRequest;
import ru.acr.altda.api.dto.ProgramResponse;
import ru.acr.altda.api.dto.ProgramSummaryResponse;
import ru.acr.altda.api.dto.RegenerateRequest;
import ru.acr.altda.api.dto.ReturnRequest;
import ru.acr.altda.api.dto.SkeletonApproveRequest;
import ru.acr.altda.service.DocxExportService;
import ru.acr.altda.service.ProgramService;
import ru.acr.altda.store.ProgramStore;

import java.util.Comparator;
import java.util.List;
import java.util.UUID;

@RestController
@RequestMapping("/programs")
public class ProgramController {

    private final ProgramService programService;
    private final ProgramMapper mapper;
    private final DocxExportService docxExportService;

    public ProgramController(
            ProgramService programService,
            ProgramMapper mapper,
            DocxExportService docxExportService
    ) {
        this.programService = programService;
        this.mapper = mapper;
        this.docxExportService = docxExportService;
    }

    @PostMapping
    public ResponseEntity<ProgramResponse> create(@Valid @RequestBody CreateProgramRequest request) {
        return ResponseEntity.status(HttpStatus.ACCEPTED).body(mapper.toResponse(programService.create(request)));
    }

    @GetMapping
    public List<ProgramSummaryResponse> list() {
        return programService.list().stream()
                .sorted(Comparator.comparing(ProgramStore.ProgramSnapshot::updatedAt).reversed())
                .map(mapper::toSummary)
                .toList();
    }

    @GetMapping("/{id}")
    public ProgramResponse get(@PathVariable UUID id) {
        return mapper.toResponse(programService.get(id));
    }

    @PostMapping("/{id}/clarifications")
    public ProgramResponse clarifications(
            @PathVariable UUID id,
            @RequestHeader(value = HttpHeaders.IF_MATCH, required = false) String ifMatch,
            @Valid @RequestBody ClarificationAnswerRequest body
    ) {
        Long expected = VersionSupport.resolveExpectedVersion(ifMatch, null);
        return mapper.toResponse(programService.answerClarifications(id, body, expected));
    }

    @PostMapping("/{id}/skeleton/approve")
    public ProgramResponse approveSkeleton(
            @PathVariable UUID id,
            @RequestHeader(value = HttpHeaders.IF_MATCH, required = false) String ifMatch,
            @Valid @RequestBody SkeletonApproveRequest body
    ) {
        Long expected = VersionSupport.resolveExpectedVersion(ifMatch, body.version());
        return mapper.toResponse(programService.approveSkeleton(id, body, expected));
    }

    @PostMapping("/{id}/skeleton/return")
    public ProgramResponse returnSkeleton(
            @PathVariable UUID id,
            @RequestHeader(value = HttpHeaders.IF_MATCH, required = false) String ifMatch,
            @Valid @RequestBody ReturnRequest body
    ) {
        Long expected = VersionSupport.resolveExpectedVersion(ifMatch, body.version());
        return mapper.toResponse(programService.returnSkeleton(id, body, expected));
    }

    @PostMapping("/{id}/content/approve")
    public ProgramResponse approveContent(
            @PathVariable UUID id,
            @RequestHeader(value = HttpHeaders.IF_MATCH, required = false) String ifMatch,
            @RequestBody(required = false) ContentApproveRequest body
    ) {
        ContentApproveRequest req = body == null ? new ContentApproveRequest(null, null) : body;
        Long expected = VersionSupport.resolveExpectedVersion(ifMatch, req.version());
        return mapper.toResponse(programService.approveContent(id, req, expected));
    }

    @PostMapping("/{id}/content/return")
    public ProgramResponse returnContent(
            @PathVariable UUID id,
            @RequestHeader(value = HttpHeaders.IF_MATCH, required = false) String ifMatch,
            @Valid @RequestBody ReturnRequest body
    ) {
        Long expected = VersionSupport.resolveExpectedVersion(ifMatch, body.version());
        return mapper.toResponse(programService.returnContent(id, body, expected));
    }

    @PostMapping("/{id}/regenerate")
    public ProgramResponse regenerate(
            @PathVariable UUID id,
            @RequestHeader(value = HttpHeaders.IF_MATCH, required = false) String ifMatch,
            @RequestBody RegenerateRequest body
    ) {
        Long expected = VersionSupport.resolveExpectedVersion(ifMatch, null);
        return mapper.toResponse(programService.regenerate(id, body == null
                ? new RegenerateRequest(null, null, "content")
                : body, expected));
    }

    @PostMapping("/{id}/lock")
    public ProgramResponse lock(
            @PathVariable UUID id,
            @RequestHeader(value = HttpHeaders.IF_MATCH, required = false) String ifMatch,
            @Valid @RequestBody LockNodesRequest body
    ) {
        Long expected = VersionSupport.resolveExpectedVersion(ifMatch, null);
        return mapper.toResponse(programService.lockNodes(id, body, expected));
    }

    @PostMapping("/{id}/allocate")
    public ProgramResponse allocate(
            @PathVariable UUID id,
            @RequestHeader(value = HttpHeaders.IF_MATCH, required = false) String ifMatch
    ) {
        Long expected = VersionSupport.resolveExpectedVersion(ifMatch, null);
        return mapper.toResponse(programService.allocate(id, expected));
    }

    @GetMapping(value = {"/{id}/export.docx", "/{id}/export/docx"}, produces = "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
    public ResponseEntity<byte[]> exportDocx(@PathVariable UUID id) {
        byte[] bytes = docxExportService.export(programService.get(id));
        return ResponseEntity.ok()
                .header(HttpHeaders.CONTENT_DISPOSITION, "attachment; filename=\"program-" + id + ".docx\"")
                .contentType(MediaType.parseMediaType(
                        "application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
                .body(bytes);
    }
}
