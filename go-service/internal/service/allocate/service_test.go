package allocate

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
)

const examplePath = "../../../../schema/example_program.json"

func loadExample(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("schema example not found: %v", err)
	}
	return data
}

func decode(t *testing.T, data []byte) *program.Program {
	t.Helper()
	var p program.Program
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func run(t *testing.T, p *program.Program) error {
	t.Helper()
	return NewService(allocator.DefaultConfig()).Allocate(context.Background(), p)
}

func half(v float64) int { return int(math.Round(v * 2)) }

func rowOf(t *testing.T, node any) map[string]float64 {
	t.Helper()
	raw := node.(map[string]any)
	out := map[string]float64{}
	for _, k := range []string{"lecture", "practice", "lab", "selfStudy", "assessment", "contact", "total"} {
		out[k] = raw[k].(float64)
	}
	return out
}

// assertHoursValid applies the validator's HOURS_* rules to the output.
func assertHoursValid(t *testing.T, out map[string]any) {
	t.Helper()

	check := func(label string, h map[string]float64) {
		if half(h["contact"]) != half(h["lecture"]+h["practice"]+h["lab"]) {
			t.Errorf("%s: HOURS_CONTACT broken: %v", label, h)
		}
		if half(h["total"]) != half(h["contact"]+h["selfStudy"]+h["assessment"]) {
			t.Errorf("%s: HOURS_ROW_SUM broken: %v", label, h)
		}
	}

	curriculum := out["curriculum"].(map[string]any)
	var modulesTotal float64
	for _, m := range curriculum["modules"].([]any) {
		module := m.(map[string]any)
		moduleRow := rowOf(t, module["hours"])
		check(module["id"].(string), moduleRow)

		var topicsTotal float64
		for _, tp := range module["topics"].([]any) {
			topic := tp.(map[string]any)
			topicRow := rowOf(t, topic["hours"])
			check(topic["id"].(string), topicRow)
			topicsTotal += topicRow["total"]
		}
		if half(topicsTotal) != half(moduleRow["total"]) {
			t.Errorf("%v: HOURS_TOPICS_EQ_MODULE broken: topics %v, module %v", module["id"], topicsTotal, moduleRow["total"])
		}
		modulesTotal += moduleRow["total"]
	}

	attestation := rowOf(t, out["finalAttestation"].(map[string]any)["hours"])
	check("finalAttestation", attestation)
	totals := rowOf(t, curriculum["totals"])
	check("totals", totals)

	if half(totals["total"]) != half(modulesTotal+attestation["total"]) {
		t.Errorf("HOURS_TOTALS_FIELD broken: totals %v, modules+attestation %v", totals["total"], modulesTotal+attestation["total"])
	}
	requested := out["header"].(map[string]any)["totalHours"].(float64)
	if half(totals["total"]) != half(requested) {
		t.Errorf("HOURS_EQ_REQUESTED broken: %v h, requested %v h", totals["total"], requested)
	}
}

func TestAllocateKeepsTheExampleValidAndUntouchedElsewhere(t *testing.T) {
	original := loadExample(t)
	p := decode(t, original)

	if err := run(t, p); err != nil {
		t.Fatal(err)
	}

	out := asMap(t, p)
	assertHoursValid(t, out)

	var before map[string]any
	if err := json.Unmarshal(original, &before); err != nil {
		t.Fatal(err)
	}
	for key, want := range before {
		if key == "curriculum" || key == "finalAttestation" {
			continue
		}
		if !reflect.DeepEqual(out[key], want) {
			t.Errorf("top-level %q was modified by the allocation", key)
		}
	}
}

func TestAllocateRespectsLockedTopicAndRebalancesTheRest(t *testing.T) {
	p := decode(t, loadExample(t))

	// A methodologist raises T1.1 to 8 h of lecture and locks it: the plan now
	// adds up to more than the requested hours.
	topic := &p.V.Curriculum.V.Modules[0].V.Topics[0]
	topic.V.Hours = program.Hours{Lecture: 8, Contact: 8, Total: 8}
	locked := true
	topic.V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}

	if err := run(t, p); err != nil {
		t.Fatal(err)
	}

	out := asMap(t, p)
	assertHoursValid(t, out)

	got := p.V.Curriculum.V.Modules[0].V.Topics[0].V.Hours
	if got.Lecture != 8 || got.Total != 8 {
		t.Errorf("locked topic changed: %+v", got)
	}
}

func TestAllocateRespectsLockedModule(t *testing.T) {
	p := decode(t, loadExample(t))

	locked := true
	p.V.Curriculum.V.Modules[1].V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}
	before := p.V.Curriculum.V.Modules[1].V.Hours

	if err := run(t, p); err != nil {
		t.Fatal(err)
	}
	assertHoursValid(t, asMap(t, p))

	if got := p.V.Curriculum.V.Modules[1].V.Hours; got != before {
		t.Errorf("locked module changed: %+v -> %+v", before, got)
	}
}

func TestAllocateRespectsLockedAttestation(t *testing.T) {
	p := decode(t, loadExample(t))

	p.V.FinalAttestation.V.Hours = program.Hours{Assessment: 3, Total: 3}
	locked := true
	p.V.FinalAttestation.V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}

	if err := run(t, p); err != nil {
		t.Fatal(err)
	}
	assertHoursValid(t, asMap(t, p))

	if got := p.V.FinalAttestation.V.Hours.Total; got != 3 {
		t.Errorf("locked attestation = %v h, want 3", got)
	}
}

func TestAllocateErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*program.Program)
		want   error
	}{
		{"no header", func(p *program.Program) { p.V.Header = nil }, ErrInvalidProgram},
		{"no curriculum", func(p *program.Program) { p.V.Curriculum = nil }, ErrInvalidProgram},
		{"no modules", func(p *program.Program) { p.V.Curriculum.V.Modules = nil }, ErrInvalidProgram},
		{"no final attestation", func(p *program.Program) { p.V.FinalAttestation = nil }, ErrInvalidProgram},
		{"zero hours", func(p *program.Program) { p.V.Header.V.TotalHours = 0 }, ErrInvalidProgram},
		{"module without topics", func(p *program.Program) { p.V.Curriculum.V.Modules[0].V.Topics = nil }, ErrInvalidProgram},
		{"more locked hours than the program has", func(p *program.Program) {
			locked := true
			topic := &p.V.Curriculum.V.Modules[0].V.Topics[0]
			topic.V.Hours = program.Hours{Lecture: 40, Contact: 40, Total: 40}
			topic.V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}
		}, ErrNotEnoughHours},
		{"everything locked and hours do not match", func(p *program.Program) {
			locked := true
			p.V.Header.V.TotalHours = 40
			p.V.FinalAttestation.V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}
			for mi := range p.V.Curriculum.V.Modules {
				p.V.Curriculum.V.Modules[mi].V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}
			}
		}, ErrNothingToBalance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := decode(t, loadExample(t))
			tt.mutate(p)

			if err := run(t, p); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAllocateFromAnEmptySkeleton(t *testing.T) {
	// Topics with no hours yet: kinds default to lecture + practice.
	in := `{"id": "p", "status": "skeleton_proposed", "request": {"rawText": "x"},
		"header": {"totalHours": 72, "studyForm": "очная"},
		"curriculum": {"modules": [
			{"id": "M1", "weight": 0.6, "hours": {}, "topics": [
				{"id": "T1.1", "weight": 0.5, "hours": {}}, {"id": "T1.2", "weight": 0.5, "hours": {}}]},
			{"id": "M2", "weight": 0.4, "hours": {}, "topics": [
				{"id": "T2.1", "weight": 1, "hours": {}}]}],
			"totals": {}},
		"finalAttestation": {"form": "зачёт", "description": "x", "hours": {}}}`

	p := decode(t, []byte(in))
	if err := run(t, p); err != nil {
		t.Fatal(err)
	}

	out := asMap(t, p)
	assertHoursValid(t, out)

	m1 := p.V.Curriculum.V.Modules[0].V.Hours.Total
	m2 := p.V.Curriculum.V.Modules[1].V.Hours.Total
	if m1 <= m2 {
		t.Errorf("0.6 vs 0.4 should give module 1 more hours, got %v and %v", m1, m2)
	}
}
