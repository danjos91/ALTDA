package ru.acr.altda.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.poi.xwpf.usermodel.XWPFDocument;
import org.apache.poi.xwpf.usermodel.XWPFParagraph;
import org.apache.poi.xwpf.usermodel.XWPFStyles;
import org.apache.poi.xwpf.usermodel.XWPFTable;
import org.apache.poi.xwpf.usermodel.XWPFTableRow;
import org.junit.jupiter.api.Test;
import org.springframework.core.io.ClassPathResource;
import ru.acr.altda.domain.ProgramStatus;
import ru.acr.altda.store.ProgramStore;

import java.io.ByteArrayInputStream;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import java.util.stream.Collectors;

import static org.assertj.core.api.Assertions.assertThat;

class DocxExportServiceTest {

    private final ObjectMapper mapper = new ObjectMapper();
    private final DocxExportService exportService = new DocxExportService();

    @Test
    void templateExists() throws Exception {
        assertThat(new ClassPathResource("templates/program-template.docx").exists()).isTrue();
    }

    @Test
    void templateStylesExist() throws Exception {
        try (XWPFDocument doc = new XWPFDocument(
                new ClassPathResource("templates/program-template.docx").getInputStream())) {
            XWPFStyles styles = doc.getStyles();
            assertThat(styles).isNotNull();
            assertThat(styles.styleExist("Heading1")).isTrue();
            assertThat(styles.styleExist("Heading2")).isTrue();
            assertThat(styles.styleExist("Title")).isTrue();
            assertThat(styles.styleExist("Normal")).isTrue();
            assertThat(styles.styleExist("TableGrid")).isTrue();
        }
    }

    @Test
    void exportContainsHoursTableWithMatchingTotals() throws Exception {
        ObjectNode program = sampleProgram(72);
        byte[] bytes = exportService.export(snapshot(program, ProgramStatus.VALIDATED));
        assertThat(bytes.length).isGreaterThan(1000);

        try (XWPFDocument doc = new XWPFDocument(new ByteArrayInputStream(bytes))) {
            assertThat(doc.getTables()).isNotEmpty();
            XWPFTable table = doc.getTables().get(0);
            assertThat(table.getStyleID()).isEqualTo("TableGrid");
            XWPFTableRow last = table.getRow(table.getNumberOfRows() - 1);
            assertThat(last.getCell(0).getText()).containsIgnoringCase("ИТОГО");
            double totalCell = Double.parseDouble(last.getCell(7).getText().replace(',', '.'));
            assertThat(totalCell).isEqualTo(72.0);

            double summed = 0;
            for (int i = 1; i < table.getNumberOfRows() - 1; i++) {
                String label = table.getRow(i).getCell(0).getText();
                if (!label.startsWith("  ")) {
                    summed += Double.parseDouble(table.getRow(i).getCell(7).getText().replace(',', '.'));
                }
            }
            assertThat(summed).isEqualTo(72.0);

            String allText = doc.getParagraphs().stream()
                    .map(XWPFParagraph::getText)
                    .collect(Collectors.joining("\n"));
            assertThat(allText).doesNotContain("ЧЕРНОВИК");
        }
    }

    @Test
    void draftExportAddsChernovikWatermark() throws Exception {
        ObjectNode program = sampleProgram(16);
        byte[] bytes = exportService.export(snapshot(program, ProgramStatus.CONTENT_READY));
        try (XWPFDocument doc = new XWPFDocument(new ByteArrayInputStream(bytes))) {
            String allText = doc.getParagraphs().stream()
                    .map(XWPFParagraph::getText)
                    .collect(Collectors.joining("\n"));
            assertThat(allText).contains("ЧЕРНОВИК");
        }
    }

    @Test
    void multilineLlmTextBecomesSeparateParagraphs() throws Exception {
        ObjectNode program = sampleProgram(16);
        program.withObject("/goal").put("text", "Первая строка цели.\nВторая строка цели.");
        byte[] bytes = exportService.export(snapshot(program, ProgramStatus.VALIDATED));
        try (XWPFDocument doc = new XWPFDocument(new ByteArrayInputStream(bytes))) {
            List<String> texts = doc.getParagraphs().stream()
                    .map(XWPFParagraph::getText)
                    .toList();
            assertThat(texts).anyMatch(t -> t.contains("Первая строка цели."));
            assertThat(texts).anyMatch(t -> t.equals("Вторая строка цели."));
            assertThat(texts.stream().noneMatch(t -> t.contains("Первая строка цели.\nВторая")))
                    .isTrue();
        }
    }

    private ProgramStore.ProgramSnapshot snapshot(ObjectNode program, ProgramStatus status) {
        return new ProgramStore.ProgramSnapshot(
                UUID.fromString(program.path("id").asText()),
                status,
                false,
                null,
                null,
                1L,
                "A",
                null,
                null,
                null,
                program,
                List.of(),
                Instant.now(),
                Instant.now()
        );
    }

    private ObjectNode sampleProgram(double hours) throws Exception {
        JsonNode full = mapper.readTree(new ClassPathResource("examples/example_program.json").getInputStream());
        ObjectNode program = (ObjectNode) full.deepCopy();
        program.put("id", "11111111-1111-4111-8111-111111111111");
        program.put("programType", "DPP_PK");
        program.withObject("/header").put("totalHours", hours);
        MockHours.allocate(program, hours);
        return program;
    }

    static final class MockHours {
        static void allocate(ObjectNode program, double totalHours) {
            var mods = program.withObject("/curriculum").withArray("modules");
            double fa = 4;
            double rem = totalHours - fa;
            double per = rem / Math.max(1, mods.size());
            for (int i = 0; i < mods.size(); i++) {
                ObjectNode m = (ObjectNode) mods.get(i);
                double mh = (i == mods.size() - 1) ? rem - per * (mods.size() - 1) : per;
                put(m.putObject("hours"), mh * 0.4, mh * 0.6, 0, 0, 0);
                var topics = m.withArray("topics");
                for (int t = 0; t < topics.size(); t++) {
                    ObjectNode topic = (ObjectNode) topics.get(t);
                    double th = mh / Math.max(1, topics.size());
                    put(topic.putObject("hours"), th * 0.4, th * 0.6, 0, 0, 0);
                }
            }
            put(program.withObject("/finalAttestation").putObject("hours"), 0, 0, 0, 0, fa);
            ObjectNode totals = program.withObject("/curriculum").putObject("totals");
            double lecture = 0, practice = 0, contact = 0, total = 0;
            for (JsonNode m : mods) {
                lecture += m.path("hours").path("lecture").asDouble();
                practice += m.path("hours").path("practice").asDouble();
                contact += m.path("hours").path("contact").asDouble();
                total += m.path("hours").path("total").asDouble();
            }
            total += fa;
            totals.put("lecture", lecture);
            totals.put("practice", practice);
            totals.put("lab", 0);
            totals.put("selfStudy", 0);
            totals.put("assessment", fa);
            totals.put("contact", contact);
            totals.put("total", total);
        }

        private static void put(ObjectNode h, double lec, double prac, double lab, double self, double assess) {
            double contact = lec + prac + lab;
            h.put("lecture", Math.round(lec * 2) / 2.0);
            h.put("practice", Math.round(prac * 2) / 2.0);
            h.put("lab", lab);
            h.put("selfStudy", self);
            h.put("assessment", assess);
            h.put("contact", Math.round(contact * 2) / 2.0);
            h.put("total", Math.round((contact + self + assess) * 2) / 2.0);
        }
    }
}
