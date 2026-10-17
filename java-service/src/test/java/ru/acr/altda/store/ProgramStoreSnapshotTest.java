package ru.acr.altda.store;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.junit.jupiter.api.Test;
import ru.acr.altda.domain.ProgramRecord;

import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Review item 3: GET snapshot is a deepCopy; mutating the snapshot does not affect the store.
 */
class ProgramStoreSnapshotTest {

    private final ObjectMapper mapper = new ObjectMapper();

    @Test
    void snapshotIsDeepCopy_notLiveNode() {
        ProgramStore store = new ProgramStore();
        UUID id = UUID.randomUUID();
        ObjectNode program = mapper.createObjectNode();
        program.put("id", id.toString());
        program.putObject("header").put("title", "Original");
        store.saveNew(new ProgramRecord(id, program));

        ProgramStore.ProgramSnapshot snap = store.snapshot(id);
        ((ObjectNode) snap.program().withObject("/header")).put("title", "Mutated snapshot");

        assertThat(store.programCopy(id).path("header").path("title").asText()).isEqualTo("Original");
        assertThat(snap.program().path("header").path("title").asText()).isEqualTo("Mutated snapshot");
    }

    @Test
    void programCopyIsIndependent() {
        ProgramStore store = new ProgramStore();
        UUID id = UUID.randomUUID();
        ObjectNode program = mapper.createObjectNode();
        program.put("id", id.toString());
        program.put("marker", 1);
        store.saveNew(new ProgramRecord(id, program));

        ObjectNode copy = store.programCopy(id);
        copy.put("marker", 99);
        assertThat(store.programCopy(id).path("marker").asInt()).isEqualTo(1);
    }
}
