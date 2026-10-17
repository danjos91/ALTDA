package ru.acr.altda.go;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.networknt.schema.JsonSchema;
import com.networknt.schema.JsonSchemaFactory;
import com.networknt.schema.SpecVersion;
import com.networknt.schema.ValidationMessage;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.springframework.core.io.ClassPathResource;

import java.io.InputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Set;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Ensures mock fixtures and mock Go responses stay aligned with program.schema.json.
 */
class MockGoClientSchemaTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static JsonSchema programSchema;
    private static JsonSchema skeletonResponseSchema;

    @BeforeAll
    static void loadSchemas() throws Exception {
        Path schemaPath = resolveSchemaPath();
        JsonNode schemaNode = MAPPER.readTree(Files.readString(schemaPath));
        JsonSchemaFactory factory = JsonSchemaFactory.getInstance(SpecVersion.VersionFlag.V202012);
        programSchema = factory.getSchema(schemaNode);

        ObjectNode skeletonRoot = MAPPER.createObjectNode();
        skeletonRoot.put("$schema", "https://json-schema.org/draft/2020-12/schema");
        skeletonRoot.setAll((ObjectNode) schemaNode.get("$defs").get("skeletonResponse"));
        // Inline $defs needed by skeletonDraft refs
        skeletonRoot.set("$defs", schemaNode.get("$defs").deepCopy());
        skeletonResponseSchema = factory.getSchema(skeletonRoot);
    }

    @Test
    void exampleProgramValidatesAgainstSchema() throws Exception {
        JsonNode example = readClasspath("examples/example_program.json");
        Set<ValidationMessage> errors = programSchema.validate(example);
        assertThat(errors)
                .as("example_program.json should validate: %s", errors)
                .isEmpty();
    }

    @Test
    void exampleSkeletonResponseValidates() throws Exception {
        JsonNode example = readClasspath("examples/example_skeleton_response.json");
        Set<ValidationMessage> errors = skeletonResponseSchema.validate(example);
        assertThat(errors)
                .as("example_skeleton_response.json should validate: %s", errors)
                .isEmpty();
    }

    @Test
    void mockGenerateContentProducesHourConsistentProgramAndIssues() throws Exception {
        MockGoClient client = new MockGoClient(MAPPER);
        client.loadExamples();

        ObjectNode seed = MAPPER.createObjectNode();
        seed.put("id", "00000000-0000-4000-8000-000000000099");
        seed.put("programType", "DPP_PK");
        ObjectNode header = seed.putObject("header");
        header.put("totalHours", 72);
        ObjectNode request = seed.putObject("request");
        request.put("rawText", "ДПП ПК Анализ данных на Python, 72 ч");
        ObjectNode params = request.putObject("params");
        params.put("programType", "DPP_PK");
        params.put("totalHours", 72);
        params.put("title", "Анализ данных на Python");

        GoClient.ContentGenerateResult result = client.generateContent(seed, null);
        ObjectNode program = result.program();

        assertThat(program.path("header").path("totalHours").asDouble()).isEqualTo(72.0);
        assertThat(program.path("curriculum").path("totals").path("total").asDouble()).isEqualTo(72.0);
        assertThat(result.issues()).isNotEmpty();
        assertThat(result.issues().get(0).path("ruleId").asText()).isEqualTo("SOURCE_UNVERIFIED");
        assertThat(result.issues().get(0).path("path").asText()).startsWith("/");

        GoClient.ValidateResult validation = client.validate(program);
        boolean hoursOk = true;
        for (JsonNode issue : validation.issues()) {
            if ("HOURS_EQ_REQUESTED".equals(issue.path("ruleId").asText())
                    && "error".equals(issue.path("severity").asText())) {
                hoursOk = false;
            }
        }
        assertThat(hoursOk).isTrue();
    }

    @Test
    void mockSkeletonGenerateAllocatesHoursOnVariants() throws Exception {
        MockGoClient client = new MockGoClient(MAPPER);
        client.loadExamples();
        ObjectNode seed = MAPPER.createObjectNode();
        seed.putObject("request").putObject("params").put("totalHours", 72);
        seed.putObject("header").put("totalHours", 72);

        GoClient.SkeletonGenerateResult result = client.generateSkeleton(seed);
        JsonNode first = result.variants().get(0);
        assertThat(first.path("modules").get(0).path("hours").path("total").asDouble()).isPositive();
        assertThat(first.path("totals").path("total").asDouble()).isEqualTo(72.0);
        assertThat(first.path("finalAttestation").path("hours").path("total").asDouble()).isPositive();
    }

    @Test
    void contentWithNodeIdsSkipsLockedDoesNotThrow() throws Exception {
        MockGoClient client = new MockGoClient(MAPPER);
        client.loadExamples();
        ObjectNode program = MAPPER.createObjectNode();
        program.put("id", "p1");
        program.putObject("header").put("totalHours", 16);
        ObjectNode module = program.putObject("curriculum").putArray("modules").addObject();
        module.put("id", "M1");
        module.putObject("_meta").put("locked", true);

        ArrayNode nodeIds = MAPPER.createArrayNode().add("M1").add("M2");
        GoClient.ContentGenerateResult result = client.generateContent(program, nodeIds);
        assertThat(result.issues().isArray()).isTrue();
        JsonNode skipped = result.program().path("generation").path("skippedLockedNodeIds");
        assertThat(skipped.isArray()).isTrue();
        assertThat(skipped.toString()).contains("M1");
    }

    @Test
    void regenerateLockedNodeReturnsNodeLocked() throws Exception {
        MockGoClient client = new MockGoClient(MAPPER);
        client.loadExamples();
        ObjectNode program = MAPPER.createObjectNode();
        program.put("id", "p1");
        ObjectNode module = program.putObject("curriculum").putArray("modules").addObject();
        module.put("id", "M1");
        module.putObject("_meta").put("locked", true);

        ArrayNode nodeIds = MAPPER.createArrayNode().add("M1");
        try {
            client.regenerateSkeleton(program, "fix", nodeIds, true);
            throw new AssertionError("expected GoClientException");
        } catch (GoClientException ex) {
            assertThat(ex.getErrorCode()).isEqualTo(GoClientException.NODE_LOCKED);
            assertThat(ex.getHttpStatus()).isEqualTo(409);
        }
    }

    @Test
    void mockParseDetectsDoopAndHours() throws Exception {
        MockGoClient client = new MockGoClient(MAPPER);
        client.loadExamples();
        GoClient.ParseResult parsed = client.parse("ДООП Робототехника, 24 ч", null);
        assertThat(parsed.params().path("programType").asText()).isEqualTo("DOOP_GENERAL");
        assertThat(parsed.params().path("totalHours").asDouble()).isEqualTo(24.0);
        assertThat(parsed.missing()).isNotEmpty();
    }

    private static JsonNode readClasspath(String path) throws Exception {
        try (InputStream in = new ClassPathResource(path).getInputStream()) {
            return MAPPER.readTree(in);
        }
    }

    private static Path resolveSchemaPath() throws Exception {
        Path testResource = Path.of("src", "test", "resources", "schema", "program.schema.json");
        if (Files.exists(testResource)) {
            return testResource.toAbsolutePath().normalize();
        }
        ClassPathResource classpath = new ClassPathResource("schema/program.schema.json");
        if (classpath.exists()) {
            return classpath.getFile().toPath();
        }
        Path fromModule = Path.of("..", "schema", "program.schema.json").toAbsolutePath().normalize();
        if (Files.exists(fromModule)) {
            return fromModule;
        }
        throw new IllegalStateException("Cannot find schema/program.schema.json");
    }
}
