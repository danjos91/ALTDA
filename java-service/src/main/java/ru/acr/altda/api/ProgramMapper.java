package ru.acr.altda.api;

import org.springframework.stereotype.Component;
import ru.acr.altda.api.dto.ProgramResponse;
import ru.acr.altda.api.dto.ProgramSummaryResponse;
import ru.acr.altda.store.ProgramStore;

@Component
public class ProgramMapper {

    public ProgramResponse toResponse(ProgramStore.ProgramSnapshot s) {
        return new ProgramResponse(
                s.id(),
                s.status().jsonValue(),
                s.busy(),
                s.error(),
                s.failedFrom() == null ? null : s.failedFrom().jsonValue(),
                s.version(),
                s.selectedVariantKey(),
                s.skeletonVariants(),
                s.validationReport(),
                s.issues(),
                s.program(),
                s.history(),
                s.createdAt(),
                s.updatedAt()
        );
    }

    public ProgramSummaryResponse toSummary(ProgramStore.ProgramSnapshot s) {
        String title = s.program().path("header").path("title").asText(null);
        if (title == null) {
            title = s.program().path("request").path("params").path("title").asText("(без названия)");
        }
        return new ProgramSummaryResponse(
                s.id(),
                s.status().jsonValue(),
                title,
                s.busy(),
                s.error(),
                s.version(),
                s.createdAt(),
                s.updatedAt()
        );
    }
}
