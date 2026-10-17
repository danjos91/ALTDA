package skeleton

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

// approvedProgram builds the program Java holds after approving variant 0 of
// the skeleton: the variant's structure and hours merged into the request.
func approvedProgram(t *testing.T, hours float64, extraParams string) *program.Program {
	t.Helper()

	params := fmt.Sprintf(`{"programType": "DOOP_GENERAL", "title": "Анализ данных на Python", "domain": "Анализ данных",
		"totalHours": %v, "audience": "Аналитики", "entryLevel": "Базовый", "studyForm": "очная"%s}`, hours, extraParams)
	variants := generate(t, params)
	v := variants[0]

	var paramsMap map[string]any
	_ = json.Unmarshal([]byte(params), &paramsMap)
	data, err := json.Marshal(map[string]any{
		"id": "p", "status": "skeleton_approved", "programType": "DOOP_GENERAL",
		"request": map[string]any{"rawText": "x", "params": paramsMap},
		"header": map[string]any{"title": "Анализ данных на Python", "itDirection": "Анализ данных",
			"totalHours": hours, "studyForm": paramsMap["studyForm"]},
		"competencies": v.Competencies,
		"curriculum":   map[string]any{"modules": v.Modules, "totals": v.Totals},
		"finalAttestation": map[string]any{"form": v.FinalAttestation.Form, "description": "x",
			"outcomeIds": v.FinalAttestation.OutcomeIDs, "hours": v.FinalAttestation.Hours},
	})
	if err != nil {
		t.Fatal(err)
	}
	var p program.Program
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func regenerate(t *testing.T, p *program.Program, in RegenerateParams) *RegenerateResult {
	t.Helper()
	result, err := newService().Regenerate(context.Background(), p, in)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func lockModule(p *program.Program, index int) {
	locked := true
	p.V.Curriculum.V.Modules[index].V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}
}

func lockTopic(p *program.Program, mi, ti int) {
	locked := true
	p.V.Curriculum.V.Modules[mi].V.Topics[ti].V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}
}

func allHours(p *program.Program) map[string]program.Hours {
	rows := map[string]program.Hours{}
	for _, m := range p.V.Curriculum.V.Modules {
		rows[m.V.ID] = m.V.Hours
		for _, t := range m.V.Topics {
			rows[t.V.ID] = t.V.Hours
		}
	}
	return rows
}

func moduleTitles(p *program.Program) []string {
	var titles []string
	for _, m := range p.V.Curriculum.V.Modules {
		titles = append(titles, m.V.Title)
	}
	return titles
}

func TestBeforeThePlanExistsItReturnsFreshVariants(t *testing.T) {
	var p program.Program
	if err := json.Unmarshal([]byte(`{"id": "p", "request": {"rawText": "x", "params": `+paramsFor(72, "")+`}}`), &p); err != nil {
		t.Fatal(err)
	}

	result := regenerate(t, &p, RegenerateParams{Comment: "Хочу больше практики", RespectLocks: true})

	if result.Program != nil {
		t.Error("variants mode must not return a program")
	}
	if len(result.Variants) != 3 {
		t.Fatalf("variants = %d, want 3", len(result.Variants))
	}
	if result.ChangedPaths == nil || len(result.ChangedPaths) != 0 {
		t.Errorf("changedPaths = %v, want an empty list", result.ChangedPaths)
	}
	for _, v := range result.Variants {
		if v.Totals.Total != 72 {
			t.Errorf("variant %s: total = %v, want 72", v.Key, v.Totals.Total)
		}
	}
}

// commentRecorder is a drafter that remembers the comment it was given.
type commentRecorder struct {
	Drafter
	comment string
}

func (c *commentRecorder) Draft(ctx context.Context, req Request) ([]skeleton.Draft, error) {
	c.comment = req.Comment
	return c.Drafter.Draft(ctx, req)
}

func TestTheCommentReachesTheDrafter(t *testing.T) {
	recorder := &commentRecorder{Drafter: NewTemplateDrafter()}
	svc := NewService(recorder, NewTemplateRegenerator(), allocator.DefaultConfig())

	var p program.Program
	if err := json.Unmarshal([]byte(`{"id": "p", "request": {"rawText": "x", "params": `+paramsFor(72, "")+`}}`), &p); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Regenerate(context.Background(), &p, RegenerateParams{Comment: "Хочу больше практики", RespectLocks: true}); err != nil {
		t.Fatal(err)
	}

	if recorder.comment != "Хочу больше практики" {
		t.Errorf("the drafter received %q", recorder.comment)
	}
}

func TestRebuildingAModuleKeepsTheProgramConsistent(t *testing.T) {
	p := approvedProgram(t, 72, "")
	before := allHours(p)
	beforeTitles := moduleTitles(p)

	result := regenerate(t, p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: true})

	if result.Variants != nil || result.Program == nil {
		t.Fatal("program mode must return the program and no variants")
	}
	if got := p.V.Curriculum.V.Totals.Total; got != 72 {
		t.Errorf("totals.total = %v, want exactly 72", got)
	}

	// The template rebuilds M2 as it was, so every row ends up where it began.
	for id, hours := range allHours(p) {
		if before[id] != hours {
			t.Errorf("%s: hours changed from %+v to %+v", id, before[id], hours)
		}
	}
	for i, title := range moduleTitles(p) {
		if beforeTitles[i] != title {
			t.Errorf("module %d title changed to %q", i+1, title)
		}
	}
	if len(result.ChangedPaths) != 1 || result.ChangedPaths[0] != "/curriculum/modules/1" {
		t.Errorf("changedPaths = %v, want [/curriculum/modules/1]", result.ChangedPaths)
	}

	report, err := validate.NewService().Validate(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if issues := report.Issues(); len(issues) != 0 {
		t.Errorf("the rebuilt program fails validation: %+v", issues)
	}
}

func TestARebuiltModuleReplacesWhatSomeoneChanged(t *testing.T) {
	p := approvedProgram(t, 72, "")
	original := p.V.Curriculum.V.Modules[1].V.Title

	module := &p.V.Curriculum.V.Modules[1].V
	module.Title = "Мой собственный модуль"
	module.Topics[0].V.Title = "Моя собственная тема"

	regenerate(t, p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: true})

	if got := p.V.Curriculum.V.Modules[1].V.Title; got != original {
		t.Errorf("module title = %q, want the template's %q", got, original)
	}
	if strings.Contains(p.V.Curriculum.V.Modules[1].V.Topics[0].V.Title, "собственная") {
		t.Error("the topic was not rebuilt")
	}
}

func TestTopicsAreWrittenWithoutTheDraftOnlyFields(t *testing.T) {
	p := approvedProgram(t, 72, "")
	regenerate(t, p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: true})

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Curriculum struct {
			Modules []struct {
				Topics []map[string]any `json:"topics"`
			} `json:"modules"`
		} `json:"curriculum"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}

	for _, topic := range out.Curriculum.Modules[1].Topics {
		for _, field := range []string{"kinds", "practiceShareHint"} {
			if _, present := topic[field]; present {
				t.Errorf("rebuilt topic %v carries %q, which the program schema does not allow", topic["id"], field)
			}
		}
		if topic["hours"] == nil || topic["outcomeIds"] == nil {
			t.Errorf("rebuilt topic is incomplete: %v", topic)
		}
	}
}

func TestOtherModulesAreUntouched(t *testing.T) {
	p := approvedProgram(t, 72, "")
	first := p.V.Curriculum.V.Modules[0]
	before, _ := json.Marshal(first)

	regenerate(t, p, RegenerateParams{NodeIDs: []string{"M3"}, RespectLocks: true})

	after, _ := json.Marshal(p.V.Curriculum.V.Modules[0])
	if string(before) != string(after) {
		t.Error("M1 changed although only M3 was rebuilt")
	}
}

func TestATopicIDMeansItsModule(t *testing.T) {
	p := approvedProgram(t, 72, "")
	p.V.Curriculum.V.Modules[1].V.Title = "Изменённое название"

	result := regenerate(t, p, RegenerateParams{NodeIDs: []string{"T2.1"}, RespectLocks: true})

	if p.V.Curriculum.V.Modules[1].V.Title == "Изменённое название" {
		t.Error("asking for a topic should rebuild its module")
	}
	if len(result.ChangedPaths) == 0 || result.ChangedPaths[0] != "/curriculum/modules/1" {
		t.Errorf("changedPaths = %v", result.ChangedPaths)
	}
}

func TestLocks(t *testing.T) {
	t.Run("a locked topic inside an explicitly requested module gives node_locked", func(t *testing.T) {
		p := approvedProgram(t, 72, "")
		lockTopic(p, 1, 0)

		_, err := newService().Regenerate(context.Background(), p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: true})
		if !errors.Is(err, ErrNodeLocked) {
			t.Fatalf("err = %v, want ErrNodeLocked", err)
		}
		if !strings.Contains(err.Error(), "M2") {
			t.Errorf("the error should name the node: %v", err)
		}
	})

	t.Run("a locked module gives node_locked", func(t *testing.T) {
		p := approvedProgram(t, 72, "")
		lockModule(p, 1)

		_, err := newService().Regenerate(context.Background(), p, RegenerateParams{NodeIDs: []string{"T2.1"}, RespectLocks: true})
		if !errors.Is(err, ErrNodeLocked) {
			t.Errorf("err = %v, want ErrNodeLocked", err)
		}
	})

	t.Run("a refused request changes nothing", func(t *testing.T) {
		p := approvedProgram(t, 72, "")
		lockTopic(p, 1, 0)
		before, _ := json.Marshal(p)

		_, _ = newService().Regenerate(context.Background(), p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: true})

		after, _ := json.Marshal(p)
		if string(before) != string(after) {
			t.Error("the program changed although the request was refused")
		}
	})

	t.Run("without respectLocks the lock is overridden", func(t *testing.T) {
		p := approvedProgram(t, 72, "")
		p.V.Curriculum.V.Modules[1].V.Title = "Изменённое название"
		lockTopic(p, 1, 0)

		regenerate(t, p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: false})

		if p.V.Curriculum.V.Modules[1].V.Title == "Изменённое название" {
			t.Error("the module was not rebuilt")
		}
	})

	t.Run("a locked module that was only inferred is skipped, not refused", func(t *testing.T) {
		p := approvedProgram(t, 72, "")
		p.V.Curriculum.V.Modules[1].V.Title = "Изменённое название"
		lockModule(p, 1)

		result := regenerate(t, p, RegenerateParams{Comment: "Упрости модуль 2", RespectLocks: true})

		if p.V.Curriculum.V.Modules[1].V.Title != "Изменённое название" {
			t.Error("the locked module was rebuilt")
		}
		if result.Program == nil || len(result.ChangedPaths) != 0 {
			t.Errorf("result = %+v, want the program back with nothing changed", result)
		}
	})

	t.Run("a locked topic elsewhere keeps its hours and the total stays exact", func(t *testing.T) {
		p := approvedProgram(t, 72, "")
		topic := &p.V.Curriculum.V.Modules[0].V.Topics[0].V
		topic.Hours = program.Hours{Lecture: 4, Practice: 4, Contact: 8, Total: 8}
		lockTopic(p, 0, 0)

		regenerate(t, p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: true})

		if got := p.V.Curriculum.V.Modules[0].V.Topics[0].V.Hours; got.Lecture != 4 || got.Practice != 4 {
			t.Errorf("the locked topic's hours changed: %+v", got)
		}
		if got := p.V.Curriculum.V.Totals.Total; got != 72 {
			t.Errorf("totals.total = %v, want exactly 72", got)
		}
	})
}

func TestTheTargetsAreInferredFromTheComment(t *testing.T) {
	tests := []struct {
		comment string
		want    []string // titles that must have been rebuilt, by module index
		modules []int
	}{
		{"Упрости модуль 2", nil, []int{1}},
		{"Переделай модуль 1 и модуль 3", nil, []int{0, 2}},
		{"Тему 2.1 лучше заменить", nil, []int{1}},
		{"Проблема в M3", nil, []int{2}},
		{"в T1.2 мало практики", nil, []int{0}},
		{"Сделай всё проще", nil, []int{0, 1, 2, 3, 4, 5}},
		{"модуль 99 не существует", nil, []int{0, 1, 2, 3, 4, 5}},
	}
	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			p := approvedProgram(t, 72, "")
			for i := range p.V.Curriculum.V.Modules {
				p.V.Curriculum.V.Modules[i].V.Title = "ИЗМЕНЕНО"
			}

			regenerate(t, p, RegenerateParams{Comment: tt.comment, RespectLocks: true})

			rebuilt := map[int]bool{}
			for i, m := range p.V.Curriculum.V.Modules {
				rebuilt[i] = m.V.Title != "ИЗМЕНЕНО"
			}
			want := map[int]bool{}
			for _, i := range tt.modules {
				want[i] = true
			}
			for i := range p.V.Curriculum.V.Modules {
				if rebuilt[i] != want[i] {
					t.Errorf("module %d: rebuilt = %v, want %v", i+1, rebuilt[i], want[i])
				}
			}
		})
	}
}

func TestExplicitNodeIDsWinOverTheComment(t *testing.T) {
	p := approvedProgram(t, 72, "")
	for i := range p.V.Curriculum.V.Modules {
		p.V.Curriculum.V.Modules[i].V.Title = "ИЗМЕНЕНО"
	}

	regenerate(t, p, RegenerateParams{Comment: "модуль 1", NodeIDs: []string{"M3"}, RespectLocks: true})

	if p.V.Curriculum.V.Modules[0].V.Title != "ИЗМЕНЕНО" || p.V.Curriculum.V.Modules[2].V.Title == "ИЗМЕНЕНО" {
		t.Error("the named node should be rebuilt and the comment ignored")
	}
}

func TestUnknownNodesAreRefused(t *testing.T) {
	for _, id := range []string{"M99", "T9.9", "banana", "C1.K1"} {
		t.Run(id, func(t *testing.T) {
			p := approvedProgram(t, 72, "")
			_, err := newService().Regenerate(context.Background(), p, RegenerateParams{NodeIDs: []string{id}, RespectLocks: true})
			if !errors.Is(err, ErrUnknownNode) {
				t.Errorf("err = %v, want ErrUnknownNode", err)
			}
		})
	}
}

func TestContentOfRebuiltTopicsIsDropped(t *testing.T) {
	p := approvedProgram(t, 72, "")

	lecture := func(topicID string) program.Node[program.Lecture] {
		return program.Node[program.Lecture]{V: program.Lecture{ID: "L" + topicID[1:], TopicID: topicID, Title: "x"}}
	}
	question := func(id, topicID string) program.Node[program.Question] {
		return program.Node[program.Question]{V: program.Question{ID: id, TopicIDs: []string{topicID}}}
	}
	task := func(id, topicID string) program.Node[program.SeminarTask] {
		return program.Node[program.SeminarTask]{V: program.SeminarTask{ID: id, TopicID: topicID}}
	}
	p.V.Lectures = []program.Node[program.Lecture]{lecture("T1.1"), lecture("T2.1")}
	p.V.Assessment = &program.Node[program.Assessment]{V: program.Assessment{
		Tests: []program.Node[program.Test]{
			{V: program.Test{ID: "TEST1", Questions: []program.Node[program.Question]{question("Q-M1-T1-1", "T1.1")}}},
			{V: program.Test{ID: "TEST2", Questions: []program.Node[program.Question]{question("Q-M2-T1-1", "T2.1")}}},
		},
		SeminarTasks: []program.Node[program.SeminarTask]{task("ST101", "T1.1"), task("ST201", "T2.1")},
	}}

	regenerate(t, p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: true})

	if len(p.V.Lectures) != 1 || p.V.Lectures[0].V.TopicID != "T1.1" {
		t.Errorf("lectures = %+v, want only T1.1's", p.V.Lectures)
	}
	if tests := p.V.Assessment.V.Tests; len(tests) != 1 || tests[0].V.ID != "TEST1" {
		t.Errorf("tests = %+v, want only TEST1 (TEST2 held only questions about the rebuilt topic)", tests)
	}
	if tasks := p.V.Assessment.V.SeminarTasks; len(tasks) != 1 || tasks[0].V.TopicID != "T1.1" {
		t.Errorf("tasks = %+v, want only T1.1's", tasks)
	}
}

func TestProgramsWithoutTheEssentialsAreRefused(t *testing.T) {
	p := approvedProgram(t, 72, "")
	p.V.FinalAttestation = nil

	_, err := newService().Regenerate(context.Background(), p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: true})
	if !errors.Is(err, ErrInvalidProgram) {
		t.Errorf("err = %v, want ErrInvalidProgram", err)
	}
}

// fakeRegenerator hands the service any proposal.
type fakeRegenerator struct {
	modules []skeleton.Module
	err     error
}

func (f fakeRegenerator) Regenerate(context.Context, RegenerateInput) ([]skeleton.Module, error) {
	return f.modules, f.err
}

func TestUnusableProposalsAreRefused(t *testing.T) {
	good := func() skeleton.Module {
		return skeleton.Module{ID: "M2", Title: "Новый", Weight: 0.2, Topics: []skeleton.Topic{
			{ID: "T2.1", Title: "a", Weight: 1, Kinds: []string{"lecture", "practice"}, OutcomeIDs: []string{"C1.K1"}, Prerequisites: []string{}},
		}}
	}
	tests := []struct {
		name string
		prop func() []skeleton.Module
		err  error
	}{
		{"a module nobody asked for", func() []skeleton.Module { m := good(); m.ID = "M3"; return []skeleton.Module{m} }, nil},
		{"the requested module is missing", func() []skeleton.Module { return nil }, nil},
		{"a module without topics", func() []skeleton.Module { m := good(); m.Topics = nil; return []skeleton.Module{m} }, nil},
		{"an unknown outcome", func() []skeleton.Module {
			m := good()
			m.Topics[0].OutcomeIDs = []string{"C9.K9"}
			return []skeleton.Module{m}
		}, nil},
		{"a topic that teaches nothing", func() []skeleton.Module {
			m := good()
			m.Topics[0].OutcomeIDs = nil
			return []skeleton.Module{m}
		}, nil},
		{"a prerequisite that comes later", func() []skeleton.Module {
			m := good()
			m.Topics[0].Prerequisites = []string{"T3.1"}
			return []skeleton.Module{m}
		}, nil},
		{"a topic ID that exists elsewhere", func() []skeleton.Module {
			m := good()
			m.Topics[0].ID = "T1.1"
			return []skeleton.Module{m}
		}, nil},
		{"the regenerator itself fails", func() []skeleton.Module { return nil }, errors.New("model unavailable")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(NewTemplateDrafter(), fakeRegenerator{modules: tt.prop(), err: tt.err}, allocator.DefaultConfig())
			p := approvedProgram(t, 72, "")
			before, _ := json.Marshal(p)

			_, err := svc.Regenerate(context.Background(), p, RegenerateParams{NodeIDs: []string{"M2"}, RespectLocks: true})

			if !errors.Is(err, ErrNoValidVariant) {
				t.Errorf("err = %v, want ErrNoValidVariant", err)
			}
			after, _ := json.Marshal(p)
			if string(before) != string(after) {
				t.Error("a refused proposal must leave the program untouched")
			}
		})
	}
}

func TestEveryVolumeStaysExactAfterARebuild(t *testing.T) {
	for _, hours := range []float64{16, 24, 40, 72, 144, 500} {
		p := approvedProgram(t, hours, "")
		regenerate(t, p, RegenerateParams{NodeIDs: []string{"M1"}, RespectLocks: true})

		if got := p.V.Curriculum.V.Totals.Total; got != hours {
			t.Errorf("%v h: totals.total = %v", hours, got)
		}
		report, err := validate.NewService().Validate(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		if issues := report.Issues(); len(issues) != 0 {
			t.Errorf("%v h: issues %+v", hours, issues)
		}
	}
}
