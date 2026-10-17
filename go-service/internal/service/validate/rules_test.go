package validate

import (
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

func node[T any](v T) program.Node[T] { return program.Node[T]{V: v} }

func topic(p *program.Program, mi, ti int) *program.Topic {
	return &p.V.Curriculum.V.Modules[mi].V.Topics[ti].V
}

// Every rule of the full program catches its own breakage on the real example.
func TestEachProgramRuleCatchesItsOwnBreakage(t *testing.T) {
	tests := []struct {
		ruleID   string
		severity Severity
		breakage func(*program.Program)
	}{
		{"CALENDAR_SESSION_NUMBERS", SeverityError, func(p *program.Program) {
			p.V.Calendar.V.Sessions[0].V.Number = 5
		}},
		{"CONSISTENCY_DANGLING_TOPIC", SeverityError, func(p *program.Program) {
			s := &p.V.Calendar.V.Sessions[0].V
			s.TopicIDs = append(s.TopicIDs, "T9.9")
		}},
		{"CALENDAR_HOURS", SeverityWarning, func(p *program.Program) {
			p.V.Calendar.V.Sessions[0].V.Hours += 1
		}},
		{"COVERAGE_OUTCOME_TAUGHT", SeverityError, func(p *program.Program) {
			c := &p.V.Competencies[0].V
			c.Knowledge = append(c.Knowledge, node(program.Outcome{ID: "C1.K9"}))
		}},
		{"COVERAGE_OUTCOME_ASSESSED", SeverityError, func(p *program.Program) {
			c := &p.V.Competencies[0].V
			c.Knowledge = append(c.Knowledge, node(program.Outcome{ID: "C1.K9"}))
			topic(p, 0, 0).OutcomeIDs = append(topic(p, 0, 0).OutcomeIDs, "C1.K9") // taught, but never assessed
		}},
		{"CONSISTENCY_OUTCOME_PREFIX", SeverityError, func(p *program.Program) {
			c := &p.V.Competencies[0].V
			c.Knowledge = append(c.Knowledge, node(program.Outcome{ID: "C2.K9"}))
		}},
		{"CONSISTENCY_DANGLING_OUTCOME", SeverityError, func(p *program.Program) {
			topic(p, 0, 0).OutcomeIDs = append(topic(p, 0, 0).OutcomeIDs, "C9.K9")
		}},
		{"ORDER_PREREQUISITES", SeverityError, func(p *program.Program) {
			topic(p, 0, 0).Prerequisites = []string{"T2.2"}
		}},
		{"CONSISTENCY_DANGLING_REFERENCE", SeverityError, func(p *program.Program) {
			topic(p, 0, 0).ReferenceIDs = []string{"REF-9"}
		}},
		{"CONSISTENCY_DANGLING_SOURCE", SeverityError, func(p *program.Program) {
			p.V.NormativeBasis = append(p.V.NormativeBasis, "SRC-99")
		}},
		{"CONSISTENCY_PROFSTANDARD_LINK", SeverityError, func(p *program.Program) {
			p.V.ProgramType = "DPP_PK"
			c := &p.V.Competencies[0].V
			c.ProfStandardLinks = []program.Node[program.ProfStandardLink]{
				node(program.ProfStandardLink{RegistryRef: "ps.unknown", LaborFunctionCode: "A/01.3"}),
			}
		}},
		{"LECTURE_MINUTES", SeverityError, func(p *program.Program) {
			p.V.Lectures[0].V.Plan[0].V.Minutes += 1
		}},
		{"TEST_KEY", SeverityError, func(p *program.Program) {
			p.V.Assessment.V.Tests[0].V.Questions[0].V.Correct = []string{"zz"}
		}},
		{"TASK_MAXSCORE", SeverityError, func(p *program.Program) {
			p.V.Assessment.V.SeminarTasks[0].V.MaxScore += 1
		}},
		{"TASK_DURATION", SeverityError, func(p *program.Program) {
			p.V.Assessment.V.SeminarTasks[0].V.DurationHours += 5
		}},
		{"TOOL_OUTDATED", SeverityWarning, func(p *program.Program) {
			p.V.Tools[0].V.Status = "outdated"
			p.V.Tools[0].V.LatestVersion = "2.0"
		}},
		{"SOURCE_UNVERIFIED", SeverityWarning, func(p *program.Program) {
			p.V.Sources[0].V.Verified = false
		}},
		{"SECTION_MISSING", SeverityError, func(p *program.Program) {
			p.V.TemplateSections[0].V.Status = "missing"
		}},
	}

	for _, tt := range tests {
		t.Run(tt.ruleID, func(t *testing.T) {
			p := load(t, examplePath)
			tt.breakage(p)

			issue := failed(validate(t, p), tt.ruleID)
			if issue == nil {
				t.Fatalf("%s did not fail", tt.ruleID)
			}
			if issue.Severity != tt.severity {
				t.Errorf("severity = %s, want %s", issue.Severity, tt.severity)
			}
			if issue.Message == "" {
				t.Error("issue has no message")
			}
		})
	}
}

func TestPathsPointAtTheOffendingNode(t *testing.T) {
	p := load(t, examplePath)
	p.V.Calendar.V.Sessions[0].V.Hours += 1
	p.V.Sources[0].V.Verified = false

	report := validate(t, p)
	if got := failed(report, "CALENDAR_HOURS").Path; got != "/calendar/sessions" {
		t.Errorf("CALENDAR_HOURS path = %q", got)
	}
	// The example's own unverified source is /sources/2; the one broken here is /sources/0.
	var paths []string
	for _, c := range report.Checks {
		if c.RuleID == "SOURCE_UNVERIFIED" && !c.Passed {
			paths = append(paths, c.Path)
		}
	}
	if len(paths) != 2 || paths[0] != "/sources/0" || paths[1] != "/sources/2" {
		t.Errorf("unverified source paths = %v, want [/sources/0 /sources/2]", paths)
	}
}

// Python's round() goes to the even neighbour on .5 (22.5 -> 22, 67.5 -> 68).
// The lecture-minutes rule must agree, or half-hour lectures would be judged wrong.
func TestLectureMinutesRoundLikeTheReferenceValidator(t *testing.T) {
	cases := []struct {
		lectureHours float64
		minutes      int
	}{
		{0.5, 22}, {1.5, 68}, {2.5, 112}, {3.5, 158}, {2, 90},
	}
	for _, tc := range cases {
		p := load(t, examplePath)
		topic(p, 0, 0).Hours.Lecture = tc.lectureHours
		p.V.Lectures[0].V.Plan = []program.Node[program.LectureStep]{node(program.LectureStep{Step: "x", Minutes: tc.minutes})}

		if issue := failed(validate(t, p), "LECTURE_MINUTES"); issue != nil {
			t.Errorf("%v lecture hours with %d min: %s", tc.lectureHours, tc.minutes, issue.Message)
		}
	}
}

func TestTestKeyRulesPerQuestionType(t *testing.T) {
	opts := func(ids ...string) []program.Node[program.Option] {
		var out []program.Node[program.Option]
		for _, id := range ids {
			out = append(out, node(program.Option{ID: id, Text: id}))
		}
		return out
	}
	tests := []struct {
		name string
		q    program.Question
		ok   bool
	}{
		{"single, one correct", program.Question{Type: "single", Options: opts("a", "b"), Correct: []string{"a"}}, true},
		{"single, two correct", program.Question{Type: "single", Options: opts("a", "b"), Correct: []string{"a", "b"}}, false},
		{"truefalse, one correct", program.Question{Type: "truefalse", Options: opts("t", "f"), Correct: []string{"t"}}, true},
		{"multiple, two of three", program.Question{Type: "multiple", Options: opts("a", "b", "c"), Correct: []string{"a", "b"}}, true},
		{"multiple, only one", program.Question{Type: "multiple", Options: opts("a", "b", "c"), Correct: []string{"a"}}, false},
		{"multiple, all of them", program.Question{Type: "multiple", Options: opts("a", "b"), Correct: []string{"a", "b"}}, false},
		{"answer not among options", program.Question{Type: "single", Options: opts("a", "b"), Correct: []string{"z"}}, false},
		{"duplicate option ids", program.Question{Type: "single", Options: opts("a", "a"), Correct: []string{"a"}}, false},
		{"short answer needs no options", program.Question{Type: "short", Correct: []string{"42"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ok, message := testKeyOK(tt.q); ok != tt.ok {
				t.Errorf("ok = %v (%s), want %v", ok, message, tt.ok)
			}
		})
	}
}

func TestAssessmentCoverageOnlyAppliesOnceContentExists(t *testing.T) {
	// At the skeleton stage nothing is assessed yet, so the rule must stay silent.
	p := skeletonStage(load(t, examplePath))

	for _, c := range validate(t, p).Checks {
		if c.RuleID == "COVERAGE_OUTCOME_ASSESSED" {
			t.Fatalf("COVERAGE_OUTCOME_ASSESSED ran at skeleton stage: %+v", c)
		}
	}

	p.V.Status = "content_ready"
	ran := false
	for _, c := range validate(t, p).Checks {
		ran = ran || c.RuleID == "COVERAGE_OUTCOME_ASSESSED"
	}
	if !ran {
		t.Error("COVERAGE_OUTCOME_ASSESSED must run from content_ready on")
	}
}
