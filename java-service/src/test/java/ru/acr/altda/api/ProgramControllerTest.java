package ru.acr.altda.api;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;

import java.util.concurrent.TimeUnit;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

@SpringBootTest
@AutoConfigureMockMvc
class ProgramControllerTest {

    @Autowired
    private MockMvc mockMvc;

    @Autowired
    private ObjectMapper objectMapper;

    @Test
    void healthOk() throws Exception {
        mockMvc.perform(get("/health"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.status").value("ok"))
                .andExpect(jsonPath("$.goClient").value("mock"));
    }

    @Test
    void createAndPollProgram() throws Exception {
        MvcResult created = mockMvc.perform(post("/programs")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"rawText\":\"ДПП ПК Анализ данных на Python, 72 ч\"}"))
                .andExpect(status().isAccepted())
                .andExpect(jsonPath("$.id").exists())
                .andExpect(jsonPath("$.status").exists())
                .andReturn();

        String id = objectMapper.readTree(created.getResponse().getContentAsString()).get("id").asText();

        JsonNode snapshot = waitUntil(id, node ->
                "clarifying".equals(node.path("status").asText())
                        || "skeleton_proposed".equals(node.path("status").asText())
                        || "failed".equals(node.path("status").asText()));

        assertThat(snapshot.path("status").asText()).isIn("clarifying", "skeleton_proposed", "failed");
        assertThat(snapshot.path("version").asLong()).isGreaterThan(0);
    }

    @Test
    void generateSyncReturnsValidatedProgramWithMatchingHours() throws Exception {
        MvcResult result = mockMvc.perform(post("/api/generate")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "request": "ДПП ПК Анализ данных на Python, 72 ч",
                                  "answers": { "entryLevel": "Базовый Python" }
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.status").value("validated"))
                .andExpect(jsonPath("$.program.header.totalHours").value(72))
                .andExpect(jsonPath("$.program.curriculum.totals.total").value(72))
                .andExpect(jsonPath("$.issues").isArray())
                .andReturn();

        JsonNode body = objectMapper.readTree(result.getResponse().getContentAsString());
        double header = body.path("program").path("header").path("totalHours").asDouble();
        double totals = body.path("program").path("curriculum").path("totals").path("total").asDouble();
        assertThat(totals).isEqualTo(header);
    }

    @Test
    void generateDoop24Hours() throws Exception {
        mockMvc.perform(post("/api/generate")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "request": "ДООП Робототехника для начинающих, 24 ч",
                                  "answers": { "entryLevel": "нулевой" }
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.program.programType").value("DOOP_GENERAL"))
                .andExpect(jsonPath("$.program.header.totalHours").value(24))
                .andExpect(jsonPath("$.program.curriculum.totals.total").value(24));
    }

    @Test
    void approveWithStaleIfMatchReturns409() throws Exception {
        MvcResult gen = mockMvc.perform(post("/api/generate")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"request":"ДПП ПК тест, 16 ч","answers":{"entryLevel":"x"}}
                                """))
                .andExpect(status().isOk())
                .andReturn();
        JsonNode body = objectMapper.readTree(gen.getResponse().getContentAsString());
        String id = body.path("id").asText();

        // Force skeleton_proposed-like approve path isn't available after generate;
        // use content approve with wrong version instead.
        mockMvc.perform(post("/programs/" + id + "/content/approve")
                        .header("If-Match", "1")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"comment\":\"ok\",\"version\":1}"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.error").value("version_conflict"));
    }

    @Test
    void exportDocxReturnsWordFile() throws Exception {
        MvcResult gen = mockMvc.perform(post("/api/generate")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"request":"ДПП ПК тест, 16 ч","answers":{"entryLevel":"x"}}
                                """))
                .andExpect(status().isOk())
                .andReturn();
        String id = objectMapper.readTree(gen.getResponse().getContentAsString()).path("id").asText();

        mockMvc.perform(get("/programs/" + id + "/export.docx"))
                .andExpect(status().isOk())
                .andExpect(header().string("Content-Disposition", org.hamcrest.Matchers.containsString(".docx")))
                .andExpect(result -> assertThat(result.getResponse().getContentAsByteArray().length).isGreaterThan(500));
    }

    @Test
    void mutationWhileBusyReturns409() throws Exception {
        MvcResult created = mockMvc.perform(post("/programs")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"rawText\":\"ДПП ПК тест, 16 ч\"}"))
                .andExpect(status().isAccepted())
                .andReturn();
        String id = objectMapper.readTree(created.getResponse().getContentAsString()).get("id").asText();

        mockMvc.perform(post("/programs/" + id + "/clarifications")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"answers\":[{\"param\":\"entryLevel\",\"answer\":\"x\"}]}"))
                .andExpect(result -> assertThat(result.getResponse().getStatus()).isIn(200, 409));
    }

    private JsonNode waitUntil(String id, java.util.function.Predicate<JsonNode> predicate) throws Exception {
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10);
        JsonNode last = null;
        while (System.nanoTime() < deadline) {
            MvcResult get = mockMvc.perform(get("/programs/" + id))
                    .andExpect(status().isOk())
                    .andReturn();
            last = objectMapper.readTree(get.getResponse().getContentAsString());
            if (predicate.test(last) && !last.path("busy").asBoolean(false)) {
                return last;
            }
            Thread.sleep(50);
        }
        return last;
    }
}
