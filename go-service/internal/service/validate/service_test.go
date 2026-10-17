package validate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocate"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
)

const (
	examplePath  = "../../../../schema/example_program.json"
	skeletonPath = "../../../../schema/example_dpp_skeleton.json"
)

func load(t *testing.T, path string) *program.Program {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("schema example not found: %v", err)
	}
	return parse(t, data)
}

func parse(t *testing.T, data []byte) *program.Program {
	t.Helper()
	var p program.Program
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func validate(t *testing.T, p *program.Program) *Report {
	t.Helper()
	report, err := NewService().Validate(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func failed(report *Report, ruleID string) *Check {
	for _, c := range report.Checks {
		if c.RuleID == ruleID && !c.Passed {
			return &c
		}
	}
	return nil
}

func errorsOf(report *Report) []Check {
	var errs []Check
	for _, c := range report.Issues() {
		if c.Severity == SeverityError {
			errs = append(errs, c)
		}
	}
	return errs
}

// The reference validator (tools/validate_reference.py) passes example_program
// with exactly one warning: its third source is unverified. This port must
// agree with it.
func TestFullExampleMatchesTheReferenceValidator(t *testing.T) {
	report := validate(t, load(t, examplePath))

	if errs := errorsOf(report); len(errs) != 0 {
		t.Errorf("unexpected errors: %+v", errs)
	}
	issues := report.Issues()
	if len(issues) != 1 || issues[0].RuleID != "SOURCE_UNVERIFIED" || issues[0].Severity != SeverityWarning {
		t.Errorf("issues = %+v, want only the SOURCE_UNVERIFIED warning", issues)
	}
	if report.Summary.UnverifiedSources != 1 || report.Summary.Warnings != 1 || report.Summary.Errors != 0 {
		t.Errorf("summary = %+v", report.Summary)
	}
}

func TestSkeletonExamplePassesEveryRule(t *testing.T) {
	report := validate(t, load(t, skeletonPath))

	if len(report.Checks) == 0 {
		t.Error("no checks were run")
	}
	if issues := report.Issues(); len(issues) != 0 {
		t.Errorf("unexpected issues: %+v", issues)
	}
}

func TestEachHoursRuleCatchesItsOwnBreakage(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		path     string
		breakage func(*program.Root)
	}{
		{"contact does not match", "HOURS_CONTACT", "/curriculum/modules/0/topics/0/hours",
			func(r *program.Root) { r.Curriculum.V.Modules[0].V.Topics[0].V.Hours.Contact += 1 }},
		{"total does not match", "HOURS_ROW_SUM", "/curriculum/modules/0/topics/0/hours",
			func(r *program.Root) { r.Curriculum.V.Modules[0].V.Topics[0].V.Hours.Total += 1 }},
		{"topics do not add up to the module", "HOURS_TOPICS_EQ_MODULE", "/curriculum/modules/0/hours",
			func(r *program.Root) { r.Curriculum.V.Modules[0].V.Hours.Lecture += 1 }},
		{"declared totals are wrong", "HOURS_TOTALS_FIELD", "/curriculum/totals",
			func(r *program.Root) { r.Curriculum.V.Totals.Lecture += 1 }},
		{"plan differs from the requested hours", "HOURS_EQ_REQUESTED", "/header/totalHours",
			func(r *program.Root) { r.Header.V.TotalHours = 20 }},
		{"practice share too low", "HOURS_PRACTICE_SHARE", "/curriculum/totals",
			func(r *program.Root) {
				share := 0.99
				r.Request.V.Params.V.MinPracticeShare = &share
			}},
		{"activities do not match the topic", "HOURS_ACTIVITIES_EQ_TOPIC", "/curriculum/modules/0/topics/0/activities",
			func(r *program.Root) { r.Curriculum.V.Modules[0].V.Topics[0].V.Activities[0].V.Hours += 1 }},
	}

	for _, tt := range tests {
		t.Run(tt.ruleID, func(t *testing.T) {
			p := load(t, examplePath)
			tt.breakage(&p.V)

			issue := failed(validate(t, p), tt.ruleID)
			if issue == nil {
				t.Fatalf("%s did not fail", tt.ruleID)
			}
			if issue.Path != tt.path {
				t.Errorf("path = %q, want %q", issue.Path, tt.path)
			}
			if issue.Severity != SeverityError || issue.Category != CategoryHours || issue.Message == "" {
				t.Errorf("issue = %+v", issue)
			}
		})
	}
}

func TestModuleLevelAssessmentIsAllowed(t *testing.T) {
	// 1 h of assessment moves from the final attestation to the first module's
	// own row. Its topics do not carry it, and the plan still adds up.
	p := load(t, examplePath)
	module := &p.V.Curriculum.V.Modules[0].V
	module.Hours.Assessment += 1
	module.Hours.Total += 1
	p.V.FinalAttestation.V.Hours = program.Hours{}

	report := validate(t, p)

	if failed(report, "HOURS_TOPICS_EQ_MODULE") != nil {
		t.Error("an assessment held only by the module row must not fail HOURS_TOPICS_EQ_MODULE")
	}
	if errs := errorsOf(report); len(errs) != 0 {
		t.Errorf("unexpected errors: %+v", errs)
	}
}

func TestNothingIsCheckedBeforeASkeletonExists(t *testing.T) {
	for _, status := range []string{"intake", "clarifying"} {
		p := parse(t, []byte(`{"id": "p", "status": "`+status+`", "request": {"rawText": "x"}}`))

		report := validate(t, p)
		if len(report.Checks) != 0 || report.Checks == nil {
			t.Errorf("%s: checks = %v, want an empty, non-nil list", status, report.Checks)
		}
		if report.Issues() == nil {
			t.Errorf("%s: Issues() must be an empty list, not nil", status)
		}
	}
}

func TestInvalidPrograms(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"unknown status", `{"id": "p", "status": "weird"}`},
		{"missing status", `{"id": "p"}`},
		{"no header", `{"status": "skeleton_proposed"}`},
		{"no curriculum", `{"status": "skeleton_proposed", "header": {"totalHours": 16}}`},
		{"no final attestation", `{"status": "skeleton_proposed", "header": {"totalHours": 16}, "curriculum": {"modules": [], "totals": {}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService().Validate(context.Background(), parse(t, []byte(tt.json)))
			if !errors.Is(err, ErrInvalidProgram) {
				t.Errorf("err = %v, want ErrInvalidProgram", err)
			}
		})
	}
}

func TestSummaryCountsPassedAndFailed(t *testing.T) {
	p := load(t, examplePath)
	p.V.Header.V.TotalHours = 20 // breaks HOURS_EQ_REQUESTED; the example's unverified source stays a warning

	report := validate(t, p)

	if report.Summary.Errors != 1 || report.Summary.Warnings != 1 {
		t.Errorf("summary = %+v, want 1 error and 1 warning", report.Summary)
	}
	if want := len(report.Checks) - 2; report.Summary.Passed != want {
		t.Errorf("passed = %d, want %d", report.Summary.Passed, want)
	}
	if len(report.Issues()) != 2 {
		t.Errorf("issues = %d, want 2", len(report.Issues()))
	}
}

func TestReportUsesTheClock(t *testing.T) {
	now := time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC)
	svc := &service{now: func() time.Time { return now }}

	report, err := svc.Validate(context.Background(), load(t, examplePath))
	if err != nil {
		t.Fatal(err)
	}
	if !report.GeneratedAt.Equal(now) {
		t.Errorf("generatedAt = %v, want %v", report.GeneratedAt, now)
	}
}

// skeletonStage drops every block the content step fills, leaving what exists
// right after the skeleton is approved.
func skeletonStage(p *program.Program) *program.Program {
	root := &p.V
	root.Status = "skeleton_approved"
	root.Lectures, root.Sources, root.NormativeBasis = nil, nil, nil
	root.Calendar, root.Assessment, root.Tools, root.TemplateSections = nil, nil, nil, nil
	for mi := range root.Curriculum.V.Modules {
		topics := root.Curriculum.V.Modules[mi].V.Topics
		for ti := range topics {
			topics[ti].V.Activities = nil
		}
	}
	return p
}

func lockTopic(topic *program.Node[program.Topic], hours program.Hours) {
	locked := true
	topic.V.Hours = hours
	topic.V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}
}

// The allocator and the validator must agree: whatever allocate produces has
// to pass every rule at the skeleton stage.
func TestAllocatedProgramsPassValidation(t *testing.T) {
	allocateSvc := allocate.NewService(allocator.DefaultConfig())

	cases := map[string]func() *program.Program{
		"example, untouched": func() *program.Program { return skeletonStage(load(t, examplePath)) },
		"example with a locked topic": func() *program.Program {
			p := skeletonStage(load(t, examplePath))
			lockTopic(&p.V.Curriculum.V.Modules[0].V.Topics[0], program.Hours{Lecture: 3, Practice: 3, Contact: 6, Total: 6})
			return p
		},
		"empty skeleton": func() *program.Program {
			return parse(t, []byte(`{"id": "p", "status": "skeleton_proposed", "request": {"rawText": "x"},
				"header": {"totalHours": 72, "studyForm": "очная"},
				"curriculum": {"modules": [
					{"id": "M1", "weight": 0.6, "hours": {}, "topics": [
						{"id": "T1.1", "weight": 0.5, "hours": {}}, {"id": "T1.2", "weight": 0.5, "hours": {}}]},
					{"id": "M2", "weight": 0.4, "hours": {}, "topics": [{"id": "T2.1", "weight": 1, "hours": {}}]}],
					"totals": {}},
				"finalAttestation": {"form": "зачёт", "description": "x", "hours": {}}}`))
		},
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			p := build()
			if err := allocateSvc.Allocate(context.Background(), p); err != nil {
				t.Fatal(err)
			}

			if issues := validate(t, p).Issues(); len(issues) != 0 {
				t.Errorf("allocated program fails validation: %+v", issues)
			}
		})
	}
}

// When a locked topic makes the requested practice share impossible, the
// allocator warns and the validator reports exactly that rule, nothing else.
func TestUnreachablePracticeShareIsReportedByTheValidator(t *testing.T) {
	p := skeletonStage(load(t, examplePath))
	lockTopic(&p.V.Curriculum.V.Modules[0].V.Topics[0], program.Hours{Lecture: 8, Contact: 8, Total: 8})

	if err := allocate.NewService(allocator.DefaultConfig()).Allocate(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	issues := validate(t, p).Issues()
	if len(issues) != 1 || issues[0].RuleID != "HOURS_PRACTICE_SHARE" {
		t.Errorf("issues = %+v, want only HOURS_PRACTICE_SHARE", issues)
	}
}

// allocate rewrites hours, not content. Whatever the content step had already
// produced for the topics that changed is now out of step with their hours, and
// the validator says so; the content for those topics must be regenerated.
func TestStaleContentIsFlaggedAfterAllocate(t *testing.T) {
	p := load(t, examplePath)
	lockTopic(&p.V.Curriculum.V.Modules[0].V.Topics[0], program.Hours{Lecture: 3, Practice: 3, Contact: 6, Total: 6})

	if err := allocate.NewService(allocator.DefaultConfig()).Allocate(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	report := validate(t, p)
	for _, ruleID := range []string{"HOURS_ACTIVITIES_EQ_TOPIC", "LECTURE_MINUTES", "CALENDAR_HOURS"} {
		if failed(report, ruleID) == nil {
			t.Errorf("%s should flag the content left behind by the reallocation", ruleID)
		}
	}
}
