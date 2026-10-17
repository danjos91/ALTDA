package ru.acr.altda.store;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import ru.acr.altda.domain.ProgramRecord;
import ru.acr.altda.domain.ProgramStatus;

import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * Review item 2: version increments once per update; setters do not bump;
 * mismatch yields OptimisticLockException (HTTP 409 via controller).
 */
class ProgramStoreVersionTest {

    private final ObjectMapper mapper = new ObjectMapper();
    private ProgramStore store;
    private UUID id;

    @BeforeEach
    void setUp() {
        store = new ProgramStore();
        id = UUID.randomUUID();
        ObjectNode program = mapper.createObjectNode();
        program.put("id", id.toString());
        program.put("status", "intake");
        store.saveNew(new ProgramRecord(id, program));
    }

    @Test
    void settersDoNotBumpVersion_updateBumpsOnce() {
        long afterSave = store.snapshot(id).version();
        store.update(id, r -> {
            r.setBusy(true);
            r.setStatus(ProgramStatus.CLARIFYING);
            r.setError("temp");
            r.clearError();
        });
        long afterUpdate = store.snapshot(id).version();
        assertThat(afterUpdate).isEqualTo(afterSave + 1);
    }

    @Test
    void expectedVersionMismatchThrows() {
        long v = store.snapshot(id).version();
        assertThatThrownBy(() -> store.updateEditable(id, v + 99, r -> r.setBusy(true)))
                .isInstanceOf(OptimisticLockException.class);
        assertThat(store.snapshot(id).version()).isEqualTo(v);
    }

    @Test
    void expectedVersionMatchSucceeds() {
        long v = store.snapshot(id).version();
        store.updateEditable(id, v, r -> r.setStatus(ProgramStatus.CLARIFYING));
        assertThat(store.snapshot(id).version()).isEqualTo(v + 1);
        assertThat(store.snapshot(id).status()).isEqualTo(ProgramStatus.CLARIFYING);
    }
}
