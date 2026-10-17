package ru.acr.altda.service;

import com.fasterxml.jackson.databind.node.ObjectNode;
import org.springframework.stereotype.Component;
import ru.acr.altda.domain.ProgramRecord;
import ru.acr.altda.go.GoClient;

/**
 * Shared helpers for applying Go parse results onto a program record.
 */
@Component
public class ProgramServiceSupport {

    public void applyParseResult(ProgramRecord record, GoClient.ParseResult parsed) {
        ObjectNode program = record.getProgram();
        ObjectNode request = program.withObject("/request");
        if (parsed.params() != null) {
            request.set("params", parsed.params().deepCopy());
        }
        if (parsed.missing() != null) {
            request.set("missing", parsed.missing().deepCopy());
        }
        if (parsed.clarifications() != null) {
            request.set("clarifications", parsed.clarifications().deepCopy());
        }
        if (parsed.gapsReport() != null) {
            request.set("gapsReport", parsed.gapsReport().deepCopy());
        }
        if (parsed.params() != null && parsed.params().hasNonNull("programType")) {
            program.put("programType", parsed.params().get("programType").asText());
        }
        record.setProgram(program);
    }
}
