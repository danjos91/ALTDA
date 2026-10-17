package ru.acr.altda.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * SkeletonMaterializer must only merge Go-provided hours — never recalculate.
 */
class SkeletonMaterializerTest {

    private final ObjectMapper mapper = new ObjectMapper();
    private final SkeletonMaterializer materializer = new SkeletonMaterializer();

    @Test
    void mergesGoProvidedHoursAndTotalsWithoutRecalculating() {
        ObjectNode current = mapper.createObjectNode();
        current.put("id", "11111111-1111-4111-8111-111111111111");
        ObjectNode params = current.putObject("request").putObject("params");
        params.put("programType", "DPP_PK");
        params.put("title", "Анализ данных");
        params.put("totalHours", 72);
        params.put("domain", "Python");

        ObjectNode variant = mapper.createObjectNode();
        variant.put("key", "A");
        variant.put("goal", "Научиться анализу данных");

        ArrayNode modules = variant.putArray("modules");
        ObjectNode m1 = modules.addObject();
        m1.put("id", "M1");
        m1.put("title", "Модуль 1");
        ObjectNode m1Hours = m1.putObject("hours");
        // Deliberately inconsistent with a naive sum so recalc would diverge
        m1Hours.put("lecture", 10);
        m1Hours.put("practice", 20);
        m1Hours.put("lab", 0);
        m1Hours.put("selfStudy", 0);
        m1Hours.put("assessment", 0);
        m1Hours.put("contact", 30);
        m1Hours.put("total", 30);

        ObjectNode topic = m1.putArray("topics").addObject();
        topic.put("id", "T1.1");
        topic.put("title", "Тема");
        ObjectNode th = topic.putObject("hours");
        th.put("lecture", 10);
        th.put("practice", 20);
        th.put("lab", 0);
        th.put("selfStudy", 0);
        th.put("assessment", 0);
        th.put("contact", 30);
        th.put("total", 30);

        ObjectNode fa = variant.putObject("finalAttestation");
        fa.put("form", "зачёт");
        ObjectNode faHours = fa.putObject("hours");
        faHours.put("lecture", 0);
        faHours.put("practice", 0);
        faHours.put("lab", 0);
        faHours.put("selfStudy", 0);
        faHours.put("assessment", 4);
        faHours.put("contact", 0);
        faHours.put("total", 4);

        // Go totals intentionally do NOT equal module total + FA (42 vs 34) —
        // materializer must copy Go totals verbatim, not re-sum.
        ObjectNode totals = variant.putObject("totals");
        totals.put("lecture", 10);
        totals.put("practice", 20);
        totals.put("lab", 0);
        totals.put("selfStudy", 0);
        totals.put("assessment", 4);
        totals.put("contact", 30);
        totals.put("total", 42);

        ObjectNode merged = materializer.mergeVariant(current, variant);

        assertThat(merged.path("curriculum").path("modules").get(0).path("hours").path("total").asDouble())
                .isEqualTo(30.0);
        assertThat(merged.path("curriculum").path("modules").get(0).path("topics").get(0)
                .path("hours").path("total").asDouble()).isEqualTo(30.0);
        assertThat(merged.path("finalAttestation").path("hours").path("total").asDouble()).isEqualTo(4.0);
        assertThat(merged.path("curriculum").path("totals").path("total").asDouble()).isEqualTo(42.0);
        assertThat(merged.path("header").path("totalHours").asDouble()).isEqualTo(72.0);
    }

    @Test
    void doesNotInventTotalsWhenGoOmitsThem() {
        ObjectNode current = mapper.createObjectNode();
        current.putObject("request").putObject("params")
                .put("programType", "DOOP_GENERAL")
                .put("title", "Робототехника");

        ObjectNode variant = mapper.createObjectNode();
        variant.put("key", "B");
        ObjectNode m = variant.putArray("modules").addObject();
        m.put("id", "M1");
        m.put("title", "Intro");
        ObjectNode h = m.putObject("hours");
        h.put("lecture", 2);
        h.put("practice", 2);
        h.put("total", 4);
        // no variant.totals

        ObjectNode merged = materializer.mergeVariant(current, variant);

        assertThat(merged.path("curriculum").path("modules").get(0).path("hours").path("total").asDouble())
                .isEqualTo(4.0);
        assertThat(merged.path("curriculum").has("totals")).isFalse();
    }
}
