package ru.acr.altda.go;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import jakarta.annotation.PostConstruct;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.core.io.ClassPathResource;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.io.InputStream;
import java.time.Instant;
import java.util.HashSet;
import java.util.Iterator;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Default stub ({@code altda.go.client=mock}): simulates the Go skills service —
 * including hour allocation on skeleton variants (Java never computes hours).
 */
@Component
@ConditionalOnProperty(name = "altda.go.client", havingValue = "mock", matchIfMissing = true)
public class MockGoClient implements GoClient {

    private static final Pattern HOURS_PATTERN = Pattern.compile(
            "(\\d+(?:[.,]\\d+)?)\\s*(?:ч|час)", Pattern.CASE_INSENSITIVE | Pattern.UNICODE_CASE);

    private final ObjectMapper objectMapper;

    private JsonNode skeletonResponse;
    private ObjectNode dppSkeleton;
    private ObjectNode fullProgram;

    public MockGoClient(ObjectMapper objectMapper) {
        this.objectMapper = objectMapper;
    }

    @PostConstruct
    void loadExamples() throws IOException {
        skeletonResponse = readTree("examples/example_skeleton_response.json");
        dppSkeleton = (ObjectNode) readTree("examples/example_dpp_skeleton.json");
        fullProgram = (ObjectNode) readTree("examples/example_program.json");
    }

    private JsonNode readTree(String classpath) throws IOException {
        ClassPathResource resource = new ClassPathResource(classpath);
        try (InputStream in = resource.getInputStream()) {
            return objectMapper.readTree(in);
        }
    }

    @Override
    public HealthResult health() {
        return new HealthResult("ok");
    }

    @Override
    public ParseResult parse(String rawText, JsonNode clarifications) {
        boolean answered = hasAnsweredClarifications(clarifications);
        boolean doop = isDoop(rawText);
        double hours = extractHours(rawText, doop ? 24 : 16);
        String title = doop
                ? guessTitle(rawText, "Общеразвивающая программа")
                : guessTitle(rawText, "ДПП повышения квалификации");

        ObjectNode params = objectMapper.createObjectNode();
        params.put("programType", doop ? "DOOP_GENERAL" : "DPP_PK");
        params.put("title", title);
        params.put("domain", doop ? "Искусственный интеллект" : "Анализ данных");
        params.put("totalHours", hours);
        params.put("audience", doop ? "Школьники / слушатели" : "Специалисты");
        params.put("studyForm", "очная");
        params.put("distanceTech", false);
        params.put("durationWeeks", Math.max(1, (int) Math.ceil(hours / 16.0)));
        params.put("sessionHours", 4);
        params.put("sessionsPerWeek", 2);
        params.put("minPracticeShare", 0.5);
        params.put("finalAttestationForm", "защита проекта");
        params.put("isInternship", false);
        params.putArray("deliverablesRequested");
        params.putArray("constraints");

        ArrayNode missing = objectMapper.createArrayNode();
        ArrayNode outClarifications = objectMapper.createArrayNode();
        ArrayNode gaps = objectMapper.createArrayNode();

        if (!answered) {
            params.putNull("entryLevel");
            missing.add("entryLevel");
            ObjectNode q = objectMapper.createObjectNode();
            q.put("param", "entryLevel");
            q.put("question", "Какой входной уровень слушателей?");
            q.putNull("answer");
            outClarifications.add(q);
        } else {
            String answer = firstAnswer(clarifications);
            params.put("entryLevel", answer != null ? answer : "Базовый");
            ObjectNode q = objectMapper.createObjectNode();
            q.put("param", "entryLevel");
            q.put("question", "Какой входной уровень слушателей?");
            q.put("answer", params.get("entryLevel").asText());
            outClarifications.add(q);
        }

        return new ParseResult(params, missing, outClarifications, gaps);
    }

    @Override
    public SkeletonGenerateResult generateSkeleton(ObjectNode program) {
        ObjectNode response = skeletonResponse.deepCopy();
        double totalHours = program.path("header").path("totalHours").asDouble(
                program.path("request").path("params").path("totalHours").asDouble(16));
        // Go allocator: attach hours to each variant before returning
        for (JsonNode variant : response.withArray("variants")) {
            allocateHoursOntoVariant((ObjectNode) variant, totalHours);
        }
        return new SkeletonGenerateResult(response.get("variants"));
    }

    @Override
    public SkeletonRegenerateResult regenerateSkeleton(
            ObjectNode program,
            String comment,
            JsonNode nodeIds,
            boolean respectLocks
    ) {
        if (respectLocks && nodeIds != null && nodeIds.isArray()) {
            Set<String> locked = collectLockedIds(program);
            for (JsonNode idNode : nodeIds) {
                String id = idNode.asText();
                if (locked.contains(id)) {
                    throw new GoClientException(
                            GoClientException.NODE_LOCKED,
                            "Node " + id + " is locked; refuse regenerate with respectLocks=true",
                            409);
                }
            }
        }

        ObjectNode updated = dppSkeleton.deepCopy();
        updated.put("schemaVersion", SCHEMA_VERSION);
        updated.put("id", program.path("id").asText());
        updated.put("status", "skeleton_proposed");
        if (program.has("request")) {
            updated.set("request", program.get("request").deepCopy());
        }
        if (comment != null && !comment.isBlank()) {
            ArrayNode gaps = updated.withObject("/request").withArray("gapsReport");
            gaps.removeAll();
            gaps.add("Revision requested: " + comment);
        }
        double totalHours = program.path("header").path("totalHours").asDouble(
                program.path("request").path("params").path("totalHours").asDouble(16));
        applyConsistentHours(updated, totalHours);

        ObjectNode variants = skeletonResponse.deepCopy();
        for (JsonNode variant : variants.withArray("variants")) {
            allocateHoursOntoVariant((ObjectNode) variant, totalHours);
        }

        ArrayNode changed = objectMapper.createArrayNode();
        changed.add("/goal");
        changed.add("/curriculum");
        return new SkeletonRegenerateResult(updated, variants.get("variants"), changed);
    }

    @Override
    public AllocateResult allocate(ObjectNode program) {
        ObjectNode copy = program.deepCopy();
        double totalHours = copy.path("header").path("totalHours").asDouble(16);
        applyConsistentHours(copy, totalHours);
        copy.put("schemaVersion", SCHEMA_VERSION);
        return new AllocateResult(copy);
    }

    @Override
    public ContentGenerateResult generateContent(ObjectNode program, JsonNode nodeIds) {
        // Non-null nodeIds: regenerate those nodes + dependents, skipping locked (same as regenerate).
        // Mock fills the whole program but records skipped locked ids in generation metadata.
        ObjectNode content = fullProgram.deepCopy();
        content.put("schemaVersion", SCHEMA_VERSION);
        content.put("id", program.path("id").asText());
        content.put("status", "content_ready");

        if (program.has("request")) {
            content.set("request", program.get("request").deepCopy());
        }

        Set<String> skippedLocked = new HashSet<>();
        if (nodeIds != null && nodeIds.isArray() && !nodeIds.isEmpty()) {
            Set<String> locked = collectLockedIds(program);
            for (JsonNode idNode : nodeIds) {
                String id = idNode.asText();
                if (locked.contains(id)) {
                    skippedLocked.add(id);
                }
            }
        }

        String programType = program.path("programType").asText(
                program.path("request").path("params").path("programType").asText("DOOP_GENERAL"));
        content.put("programType", programType);

        double totalHours = program.path("header").path("totalHours").asDouble(
                program.path("request").path("params").path("totalHours").asDouble(16));

        if (program.has("header")) {
            content.set("header", program.get("header").deepCopy());
        }
        content.withObject("/header").put("totalHours", totalHours);

        if (program.has("goal")) {
            content.set("goal", program.get("goal").deepCopy());
        }
        if (program.has("competencies")) {
            content.set("competencies", program.get("competencies").deepCopy());
        }
        // Prefer curriculum already allocated by Go skeleton merge
        if (program.has("curriculum")
                && program.path("curriculum").path("totals").path("total").asDouble(0) > 0) {
            content.set("curriculum", program.get("curriculum").deepCopy());
            if (program.has("finalAttestation")) {
                content.set("finalAttestation", program.get("finalAttestation").deepCopy());
            }
        } else {
            applyConsistentHours(content, totalHours);
        }

        if ("DPP_PK".equals(programType)) {
            content.remove("doop");
            if (!content.has("dpp")) {
                ObjectNode dpp = content.putObject("dpp");
                ArrayNode standards = dpp.putArray("profStandards");
                ObjectNode ps = standards.addObject();
                ps.put("registryRef", "ps.06.001");
                ps.put("standardCode", "06.001");
                ps.put("standardTitle", "Программист");
                ps.putNull("approvalOrder");
                ps.put("verified", false);
                ps.putArray("laborFunctions")
                        .addObject()
                        .put("code", "A/01.3")
                        .put("title", "EXAMPLE ONLY");
            }
        } else {
            content.remove("dpp");
            if (!content.has("doop")) {
                content.putObject("doop").put("focus", "техническая");
            }
        }

        ObjectNode generation = content.putObject("generation");
        ArrayNode steps = generation.putArray("steps");
        ObjectNode parseStep = steps.addObject();
        parseStep.put("step", "parse");
        parseStep.put("model", "mock");
        parseStep.put("promptTokens", 1200);
        parseStep.put("completionTokens", 400);
        parseStep.put("durationMs", 320);
        ObjectNode skelStep = steps.addObject();
        skelStep.put("step", "skeleton");
        skelStep.put("model", "mock");
        skelStep.put("promptTokens", 2800);
        skelStep.put("completionTokens", 4900);
        skelStep.put("durationMs", 2100);
        ObjectNode contentStep = steps.addObject();
        contentStep.put("step", "content");
        contentStep.put("model", "mock");
        contentStep.put("promptTokens", 4200);
        contentStep.put("completionTokens", 9800);
        contentStep.put("durationMs", 1850);
        generation.put("embeddingTokens", 0);
        generation.put("costRub", 12.4);
        if (!skippedLocked.isEmpty()) {
            ArrayNode skipped = generation.putArray("skippedLockedNodeIds");
            skippedLocked.forEach(skipped::add);
        }

        // issues is always an array, never null
        ArrayNode issues = objectMapper.createArrayNode();
        ObjectNode warnIssue = issues.addObject();
        warnIssue.put("ruleId", "SOURCE_UNVERIFIED");
        warnIssue.put("severity", "warning");
        warnIssue.put("message", "Source not verified in local registry");
        warnIssue.put("path", "/sources/0");

        return new ContentGenerateResult(content, issues);
    }

    @Override
    public ValidateResult validate(ObjectNode program) {
        ObjectNode report = objectMapper.createObjectNode();
        report.put("generatedAt", Instant.now().toString());
        report.put("iterations", 0);

        double requested = program.path("header").path("totalHours").asDouble(0);
        double totals = program.path("curriculum").path("totals").path("total").asDouble(0);
        boolean hoursOk = Math.abs(requested - totals) < 0.01;

        ObjectNode summary = report.putObject("summary");
        summary.put("errors", hoursOk ? 0 : 1);
        summary.put("warnings", 1);
        summary.put("passed", hoursOk ? 12 : 11);
        summary.put("unverifiedSources", 1);

        ArrayNode checks = report.putArray("checks");
        ObjectNode hoursCheck = checks.addObject();
        hoursCheck.put("ruleId", "HOURS_EQ_REQUESTED");
        hoursCheck.put("category", "hours");
        hoursCheck.put("severity", "error");
        hoursCheck.put("passed", hoursOk);
        hoursCheck.put("message", hoursOk
                ? "Sum equals requested hours"
                : "Sum " + totals + " h != requested " + requested + " h");
        hoursCheck.put("path", "/header/totalHours");

        ObjectNode warn = checks.addObject();
        warn.put("ruleId", "SOURCE_UNVERIFIED");
        warn.put("category", "sources");
        warn.put("severity", "warning");
        warn.put("passed", false);
        warn.put("message", "Source not verified in local registry");
        warn.put("path", "/sources/0");

        ArrayNode issues = objectMapper.createArrayNode();
        if (!hoursOk) {
            ObjectNode issue = issues.addObject();
            issue.put("ruleId", "HOURS_EQ_REQUESTED");
            issue.put("severity", "error");
            issue.put("message", hoursCheck.get("message").asText());
            issue.put("path", "/header/totalHours");
        }
        ObjectNode warnIssue = issues.addObject();
        warnIssue.put("ruleId", "SOURCE_UNVERIFIED");
        warnIssue.put("severity", "warning");
        warnIssue.put("message", "Source not verified in local registry");
        warnIssue.put("path", "/sources/0");

        return new ValidateResult(report, issues);
    }

    /**
     * Go-side allocator simulation: write hours onto a skeleton variant (not Java business logic).
     */
    void allocateHoursOntoVariant(ObjectNode variant, double totalHours) {
        ObjectNode shell = objectMapper.createObjectNode();
        ObjectNode curriculum = shell.putObject("curriculum");
        ArrayNode modules = curriculum.putArray("modules");
        if (variant.has("modules") && variant.get("modules").isArray()) {
            for (JsonNode m : variant.get("modules")) {
                modules.add(m.deepCopy());
            }
        }
        ObjectNode fa = shell.putObject("finalAttestation");
        if (variant.has("finalAttestation")) {
            fa.setAll((ObjectNode) variant.get("finalAttestation").deepCopy());
        }
        applyConsistentHours(shell, totalHours);
        variant.set("modules", shell.path("curriculum").path("modules").deepCopy());
        ObjectNode faOut = (ObjectNode) variant.withObject("/finalAttestation");
        if (shell.path("finalAttestation").has("hours")) {
            faOut.set("hours", shell.path("finalAttestation").path("hours").deepCopy());
        }
        variant.set("totals", shell.path("curriculum").path("totals").deepCopy());
    }

    /**
     * Rebuild curriculum hours so module topic sums + final attestation == totalHours.
     * Used only inside this Go mock (stands in for the Go allocator).
     */
    void applyConsistentHours(ObjectNode program, double totalHours) {
        double attestation = Math.min(4.0, Math.max(1.0, Math.round(totalHours * 0.1 * 2) / 2.0));
        double remainder = totalHours - attestation;
        if (remainder < 1) {
            attestation = 1;
            remainder = totalHours - attestation;
        }

        ObjectNode curriculum = program.withObject("/curriculum");
        ArrayNode modules = curriculum.withArray("modules");
        if (modules.isEmpty()) {
            ObjectNode m = modules.addObject();
            m.put("id", "M1");
            m.put("title", "Модуль 1");
            m.put("weight", 1);
            m.put("isIntro", false);
            m.put("assessmentForm", "нет");
            m.putObject("hours");
            ArrayNode topics = m.putArray("topics");
            ObjectNode t = topics.addObject();
            t.put("id", "T1.1");
            t.put("title", "Тема 1.1");
            t.put("weight", 1);
            t.putObject("hours");
            t.putArray("outcomeIds");
            t.putArray("kinds").add("lecture").add("practice");
        }

        int moduleCount = modules.size();
        double perModule = Math.round((remainder / moduleCount) * 2) / 2.0;
        double assigned = 0;

        for (int i = 0; i < moduleCount; i++) {
            ObjectNode mod = (ObjectNode) modules.get(i);
            double modHours = (i == moduleCount - 1) ? (remainder - assigned) : perModule;
            assigned += (i == moduleCount - 1) ? 0 : perModule;
            ArrayNode topics = mod.withArray("topics");
            if (topics.isEmpty()) {
                ObjectNode t = topics.addObject();
                t.put("id", "T" + (i + 1) + ".1");
                t.put("title", "Тема");
                t.put("weight", 1);
                t.putObject("hours");
                t.putArray("outcomeIds");
            }
            int topicCount = topics.size();
            double topicAssigned = 0;
            double modLecture = 0;
            double modPractice = 0;
            for (int ti = 0; ti < topicCount; ti++) {
                ObjectNode topic = (ObjectNode) topics.get(ti);
                double share = Math.round((modHours / topicCount) * 2) / 2.0;
                double th = (ti == topicCount - 1) ? (modHours - topicAssigned) : share;
                if (ti != topicCount - 1) {
                    topicAssigned += share;
                }
                double tPractice = Math.round(th * 0.6 * 2) / 2.0;
                double tLecture = th - tPractice;
                modLecture += tLecture;
                modPractice += tPractice;
                putHours(topic.putObject("hours"), tLecture, tPractice, 0, 0, 0);
                ArrayNode acts = topic.putArray("activities");
                if (tLecture > 0) {
                    acts.addObject()
                            .put("kind", "lecture")
                            .put("hours", tLecture)
                            .put("content", "Лекционный материал по теме");
                }
                if (tPractice > 0) {
                    acts.addObject()
                            .put("kind", "practice")
                            .put("hours", tPractice)
                            .put("content", "Практические задания по теме");
                }
            }
            if (!mod.has("assessmentForm")) {
                mod.put("assessmentForm", "нет");
            }
            putHours(mod.putObject("hours"), modLecture, modPractice, 0, 0, 0);
        }

        ObjectNode fa = program.withObject("/finalAttestation");
        putHours(fa.putObject("hours"), 0, 0, 0, 0, attestation);

        double lectureSum = 0;
        double practiceSum = 0;
        double labSum = 0;
        double selfSum = 0;
        double assessSum = attestation;
        double contactSum = 0;
        for (JsonNode mod : modules) {
            JsonNode h = mod.path("hours");
            lectureSum += h.path("lecture").asDouble();
            practiceSum += h.path("practice").asDouble();
            labSum += h.path("lab").asDouble();
            selfSum += h.path("selfStudy").asDouble();
            assessSum += h.path("assessment").asDouble();
            contactSum += h.path("contact").asDouble();
        }
        ObjectNode totals = curriculum.putObject("totals");
        totals.put("lecture", lectureSum);
        totals.put("practice", practiceSum);
        totals.put("lab", labSum);
        totals.put("selfStudy", selfSum);
        totals.put("assessment", assessSum);
        // Same identities as docs/API.md and HOURS_* rules:
        // contact = lecture+practice+lab; total = contact+selfStudy+assessment
        totals.put("contact", contactSum);
        totals.put("total", contactSum + selfSum + assessSum);
    }

    /**
     * Allocator hour identities (every level):
     * contact = lecture + practice + lab; total = contact + selfStudy + assessment.
     */
    private static void putHours(
            ObjectNode hours,
            double lecture,
            double practice,
            double lab,
            double selfStudy,
            double assessment
    ) {
        double contact = lecture + practice + lab;
        hours.put("lecture", lecture);
        hours.put("practice", practice);
        hours.put("lab", lab);
        hours.put("selfStudy", selfStudy);
        hours.put("assessment", assessment);
        hours.put("contact", contact);
        hours.put("total", contact + selfStudy + assessment);
    }

    private static Set<String> collectLockedIds(JsonNode node) {
        Set<String> locked = new HashSet<>();
        collectLockedIds(node, locked);
        return locked;
    }

    private static void collectLockedIds(JsonNode node, Set<String> locked) {
        if (node == null || node.isNull()) {
            return;
        }
        if (node.isObject()) {
            if (node.path("_meta").path("locked").asBoolean(false) && node.has("id")) {
                locked.add(node.get("id").asText());
            }
            Iterator<JsonNode> values = node.elements();
            while (values.hasNext()) {
                collectLockedIds(values.next(), locked);
            }
        } else if (node.isArray()) {
            for (JsonNode child : node) {
                collectLockedIds(child, locked);
            }
        }
    }

    static double extractHours(String rawText, double fallback) {
        if (rawText == null) {
            return fallback;
        }
        Matcher m = HOURS_PATTERN.matcher(rawText);
        if (m.find()) {
            return Double.parseDouble(m.group(1).replace(',', '.'));
        }
        return fallback;
    }

    static boolean isDoop(String rawText) {
        if (rawText == null) {
            return false;
        }
        String lower = rawText.toLowerCase();
        return lower.contains("дооп")
                || lower.contains("общеразвива")
                || lower.contains("doop");
    }

    private static String guessTitle(String rawText, String fallback) {
        if (rawText == null || rawText.isBlank()) {
            return fallback;
        }
        String cleaned = rawText.replaceAll("\\s+", " ").trim();
        if (cleaned.length() > 80) {
            return cleaned.substring(0, 77) + "...";
        }
        return cleaned;
    }

    public JsonNode exampleSkeletonResponse() {
        return skeletonResponse.deepCopy();
    }

    public ObjectNode exampleDppSkeleton() {
        return dppSkeleton.deepCopy();
    }

    public ObjectNode exampleFullProgram() {
        return fullProgram.deepCopy();
    }

    private static boolean hasAnsweredClarifications(JsonNode clarifications) {
        if (clarifications == null || !clarifications.isArray() || clarifications.isEmpty()) {
            return false;
        }
        for (JsonNode c : clarifications) {
            JsonNode answer = c.get("answer");
            if (answer != null && !answer.isNull() && !answer.asText().isBlank()) {
                return true;
            }
        }
        return false;
    }

    private static String firstAnswer(JsonNode clarifications) {
        if (clarifications == null || !clarifications.isArray()) {
            return null;
        }
        for (JsonNode c : clarifications) {
            JsonNode answer = c.get("answer");
            if (answer != null && !answer.isNull() && !answer.asText().isBlank()) {
                return answer.asText();
            }
        }
        return null;
    }
}
