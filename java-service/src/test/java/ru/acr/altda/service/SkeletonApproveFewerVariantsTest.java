package ru.acr.altda.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import ru.acr.altda.api.dto.SkeletonApproveRequest;
import ru.acr.altda.domain.ProgramRecord;
import ru.acr.altda.domain.ProgramStatus;
import ru.acr.altda.go.GoClient;
import ru.acr.altda.store.ProgramStore;

import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.doNothing;

/**
 * Skeleton with 1–2 variants (not always 3) must validate for approval.
 */
@SpringBootTest
class SkeletonApproveFewerVariantsTest {

    @Autowired
    private ProgramService programService;

    @Autowired
    private ProgramStore store;

    @Autowired
    private ObjectMapper objectMapper;

    @MockBean
    private ProgramAsyncJobs asyncJobs;

    @Test
    void approveWithSingleVariantSucceeds() {
        doNothing().when(asyncJobs).runContentAndValidate(any());
        UUID id = seedProposed(1, 0);
        var snap = programService.approveSkeleton(id, new SkeletonApproveRequest("A", "ok", null), null);
        assertThat(snap.status()).isEqualTo(ProgramStatus.SKELETON_APPROVED);
        assertThat(snap.selectedVariantKey()).isEqualTo("A");
        assertThat(snap.program().path("curriculum").path("modules")).isNotEmpty();
    }

    @Test
    void approveWithTwoVariantsAndOutOfRangeRecommendedStillFindsKey() {
        doNothing().when(asyncJobs).runContentAndValidate(any());
        UUID id = seedProposed(2, 99); // recommendedIndex out of range — UI falls back; approve by key B
        var snap = programService.approveSkeleton(id, new SkeletonApproveRequest("B", null, null), null);
        assertThat(snap.status()).isEqualTo(ProgramStatus.SKELETON_APPROVED);
        assertThat(snap.selectedVariantKey()).isEqualTo("B");
    }

    @Test
    void wrapVariantsPreservesRecommendedIndex() {
        ObjectNode raw = objectMapper.createObjectNode();
        ArrayNode arr = raw.putArray("variants");
        arr.addObject().put("key", "A");
        raw.put("recommendedIndex", 0);
        ProgramJsonHelpers helpers = new ProgramJsonHelpers(objectMapper);
        var wrapped = helpers.wrapVariants(raw);
        assertThat(wrapped.path("recommendedIndex").asInt()).isEqualTo(0);
        assertThat(wrapped.path("variants")).hasSize(1);
    }

    private UUID seedProposed(int variantCount, int recommendedIndex) {
        UUID id = UUID.randomUUID();
        ObjectNode program = objectMapper.createObjectNode();
        program.put("id", id.toString());
        program.put("schemaVersion", "1.1");
        program.put("status", "skeleton_proposed");
        ObjectNode params = program.putObject("request").putObject("params");
        params.put("programType", "DOOP_GENERAL");
        params.put("title", "Тест");
        params.put("domain", "IT");
        params.put("totalHours", 16);
        params.put("studyForm", "очная");

        ObjectNode variantsWrap = objectMapper.createObjectNode();
        ArrayNode variants = variantsWrap.putArray("variants");
        String[] keys = {"A", "B", "C"};
        for (int i = 0; i < variantCount; i++) {
            variants.add(minimalVariant(keys[i]));
        }
        variantsWrap.put("recommendedIndex", recommendedIndex);

        ProgramRecord record = new ProgramRecord(id, program);
        record.setStatus(ProgramStatus.SKELETON_PROPOSED);
        record.setSkeletonVariants(variantsWrap);
        record.setBusy(false);
        store.saveNew(record);
        return id;
    }

    private ObjectNode minimalVariant(String key) {
        ObjectNode v = objectMapper.createObjectNode();
        v.put("key", key);
        v.put("rationale", "r");
        v.put("goal", "g");
        v.putArray("pros").add("p");
        v.putArray("cons").add("c");
        v.putArray("competencies");
        ObjectNode m = v.putArray("modules").addObject();
        m.put("id", "M1");
        m.put("title", "Модуль");
        ObjectNode mh = m.putObject("hours");
        mh.put("lecture", 8);
        mh.put("practice", 8);
        mh.put("lab", 0);
        mh.put("selfStudy", 0);
        mh.put("assessment", 0);
        mh.put("contact", 16);
        mh.put("total", 16);
        ObjectNode t = m.putArray("topics").addObject();
        t.put("id", "T1.1");
        t.put("title", "Тема");
        ObjectNode th = t.putObject("hours");
        th.put("lecture", 8);
        th.put("practice", 8);
        th.put("lab", 0);
        th.put("selfStudy", 0);
        th.put("assessment", 0);
        th.put("contact", 16);
        th.put("total", 16);
        ObjectNode fa = v.putObject("finalAttestation");
        fa.put("form", "зачёт");
        ObjectNode fah = fa.putObject("hours");
        fah.put("lecture", 0);
        fah.put("practice", 0);
        fah.put("lab", 0);
        fah.put("selfStudy", 0);
        fah.put("assessment", 0);
        fah.put("contact", 0);
        fah.put("total", 0);
        ObjectNode totals = v.putObject("totals");
        totals.put("lecture", 8);
        totals.put("practice", 8);
        totals.put("lab", 0);
        totals.put("selfStudy", 0);
        totals.put("assessment", 0);
        totals.put("contact", 16);
        totals.put("total", 16);
        return v;
    }
}
