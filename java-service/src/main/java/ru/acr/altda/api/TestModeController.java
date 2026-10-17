package ru.acr.altda.api;

import jakarta.validation.Valid;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;
import ru.acr.altda.api.dto.GenerateRequest;
import ru.acr.altda.api.dto.GenerateResponse;
import ru.acr.altda.api.dto.HealthResponse;
import ru.acr.altda.service.ProgramService;

/**
 * Simple endpoints for hackathon automated tests and local smoke checks.
 */
@RestController
public class TestModeController {

    private final ProgramService programService;
    private final String goClientMode;

    public TestModeController(
            ProgramService programService,
            @Value("${altda.go.client:mock}") String goClientMode
    ) {
        this.programService = programService;
        this.goClientMode = goClientMode;
    }

    @GetMapping("/health")
    public HealthResponse health() {
        return new HealthResponse("ok", goClientMode);
    }

    /**
     * Synchronous full pipeline with auto-approval (skips human согласование).
     */
    @PostMapping("/api/generate")
    public GenerateResponse generate(@Valid @RequestBody GenerateRequest request) {
        return programService.generateSync(request);
    }
}
