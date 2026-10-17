package ru.acr.altda.store;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import ru.acr.altda.api.ConflictException;
import ru.acr.altda.domain.ProgramRecord;
import ru.acr.altda.domain.ProgramStatus;
import ru.acr.altda.domain.ProgramStatusMachine;

import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicInteger;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Review item 1: busy/status checks inside {@link ProgramStore#update} under the record lock
 * so concurrent approvals cannot both start generation.
 */
class ProgramStoreConcurrencyTest {

    private final ObjectMapper mapper = new ObjectMapper();
    private ProgramStore store;
    private UUID id;

    @BeforeEach
    void setUp() {
        store = new ProgramStore();
        id = UUID.randomUUID();
        ObjectNode program = mapper.createObjectNode();
        program.put("schemaVersion", "1.1");
        program.put("id", id.toString());
        program.put("status", "skeleton_proposed");
        ProgramRecord record = new ProgramRecord(id, program);
        record.setStatus(ProgramStatus.SKELETON_PROPOSED);
        store.saveNew(record);
    }

    @Test
    void onlyOneConcurrentApprovalCanStartGeneration() throws Exception {
        AtomicInteger started = new AtomicInteger();
        AtomicInteger rejected = new AtomicInteger();
        CountDownLatch ready = new CountDownLatch(2);
        CountDownLatch go = new CountDownLatch(1);

        Runnable attempt = () -> {
            ready.countDown();
            try {
                go.await();
                store.updateEditable(id, null, r -> {
                    if (r.getStatus() != ProgramStatus.SKELETON_PROPOSED || r.isBusy()) {
                        throw new ConflictException("already started");
                    }
                    ProgramStatusMachine.assertTransition(r.getStatus(), ProgramStatus.SKELETON_APPROVED);
                    r.setStatus(ProgramStatus.SKELETON_APPROVED);
                    r.setBusy(true);
                    started.incrementAndGet();
                });
            } catch (ConflictException ex) {
                rejected.incrementAndGet();
            } catch (InterruptedException ex) {
                Thread.currentThread().interrupt();
                rejected.incrementAndGet();
            }
        };

        Thread t1 = new Thread(attempt);
        Thread t2 = new Thread(attempt);
        t1.start();
        t2.start();
        ready.await();
        go.countDown();
        t1.join();
        t2.join();

        assertThat(started.get()).isEqualTo(1);
        assertThat(rejected.get()).isEqualTo(1);
        ProgramStore.ProgramSnapshot snap = store.snapshot(id);
        assertThat(snap.busy()).isTrue();
        assertThat(snap.status()).isEqualTo(ProgramStatus.SKELETON_APPROVED);
    }
}
