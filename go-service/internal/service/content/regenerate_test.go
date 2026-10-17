package content

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
)

const sentinel = "SENTINEL"

// markEverything overwrites the generated text, so a later run shows exactly
// which nodes it regenerated: those lose the mark, the rest keep it.
func markEverything(p *program.Program) {
	for _, ref := range topicRefs(&p.V) {
		for i := range ref.topic.Activities {
			ref.topic.Activities[i].V.Content = sentinel
		}
	}
	for i := range p.V.Lectures {
		p.V.Lectures[i].V.Title = sentinel
	}
	for ti := range p.V.Assessment.V.Tests {
		questions := p.V.Assessment.V.Tests[ti].V.Questions
		for qi := range questions {
			questions[qi].V.Text = sentinel
		}
	}
	for i := range p.V.Assessment.V.SeminarTasks {
		p.V.Assessment.V.SeminarTasks[i].V.Title = sentinel
	}
}

func isMarked(p *program.Program, topicID string) (activities, lecture, questions, task bool) {
	topic := topicByID(p, topicID)
	activities = len(topic.Activities) > 0 && topic.Activities[0].V.Content == sentinel
	for _, l := range p.V.Lectures {
		if l.V.TopicID == topicID {
			lecture = l.V.Title == sentinel
		}
	}
	for _, test := range p.V.Assessment.V.Tests {
		for _, q := range test.V.Questions {
			if len(q.V.TopicIDs) > 0 && q.V.TopicIDs[0] == topicID {
				questions = q.V.Text == sentinel
			}
		}
	}
	for _, tk := range p.V.Assessment.V.SeminarTasks {
		if tk.V.TopicID == topicID {
			task = tk.V.Title == sentinel
		}
	}
	return
}

func completeProgram(t *testing.T) *program.Program {
	t.Helper()
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)
	markEverything(p)
	return p
}

func TestRegeneratingATopicTouchesOnlyThatTopic(t *testing.T) {
	p := completeProgram(t)

	issues := generate(t, p, "T1.2")

	a, l, q, tk := isMarked(p, "T1.2")
	if a || l || q || tk {
		t.Errorf("T1.2 was not fully regenerated: activities %v, lecture %v, questions %v, task %v", a, l, q, tk)
	}
	for _, other := range []string{"T1.1", "T1.3", "T2.1"} {
		a, l, q, tk := isMarked(p, other)
		hasLecture := topicByID(p, other).Hours.Lecture > 0
		if !a || (hasLecture && !l) || !q || !tk {
			t.Errorf("%s was changed by regenerating T1.2: activities %v, lecture %v, questions %v, task %v", other, a, l, q, tk)
		}
	}
	if !onlyMissingSections(issues) {
		t.Errorf("issues after a partial regeneration = %+v", issues)
	}
}

func TestRegeneratingAModuleTouchesItsTopics(t *testing.T) {
	p := completeProgram(t)

	generate(t, p, "M1")

	for _, ref := range topicRefs(&p.V) {
		a, _, _, _ := isMarked(p, ref.topic.ID)
		inM1 := ref.module.ID == "M1"
		if inM1 && a {
			t.Errorf("%s (in M1) was not regenerated", ref.topic.ID)
		}
		if !inM1 && !a {
			t.Errorf("%s (outside M1) was regenerated", ref.topic.ID)
		}
	}
}

func TestRegeneratingAssessmentLeavesTheRestAlone(t *testing.T) {
	p := completeProgram(t)

	issues := generate(t, p, "assessment")

	for _, ref := range topicRefs(&p.V) {
		a, l, q, tk := isMarked(p, ref.topic.ID)
		if !a {
			t.Errorf("%s: activities were regenerated, only assessment should be", ref.topic.ID)
		}
		if ref.topic.Hours.Lecture > 0 && !l {
			t.Errorf("%s: the lecture was regenerated", ref.topic.ID)
		}
		if q || tk {
			t.Errorf("%s: assessment was not regenerated (questions marked %v, task marked %v)", ref.topic.ID, q, tk)
		}
	}
	if !onlyMissingSections(issues) {
		t.Errorf("issues = %+v", issues)
	}
}

func TestLockedNodesAreSkipped(t *testing.T) {
	locked := true

	t.Run("a locked topic", func(t *testing.T) {
		p := completeProgram(t)
		topicByID(p, "T1.1").Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}

		generate(t, p) // the whole program

		if a, _, q, tk := isMarked(p, "T1.1"); !a || !q || !tk {
			t.Error("the locked topic was regenerated")
		}
		if a, _, _, _ := isMarked(p, "T1.2"); a {
			t.Error("an unlocked topic was not regenerated")
		}
	})

	t.Run("a locked module", func(t *testing.T) {
		p := completeProgram(t)
		p.V.Curriculum.V.Modules[0].V.Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}

		generate(t, p)

		for _, ref := range topicRefs(&p.V) {
			a, _, _, _ := isMarked(p, ref.topic.ID)
			if (ref.module.ID == "M1") != a {
				t.Errorf("%s: marked = %v, want %v", ref.topic.ID, a, ref.module.ID == "M1")
			}
		}
	})

	t.Run("targeting a locked topic is not an error", func(t *testing.T) {
		p := completeProgram(t)
		topicByID(p, "T1.1").Meta = &program.Node[program.Meta]{V: program.Meta{Locked: &locked}}

		if _, err := newService().Generate(context.Background(), p, []string{"T1.1"}); err != nil {
			t.Errorf("err = %v, a locked target is skipped, not refused", err)
		}
		if a, _, _, _ := isMarked(p, "T1.1"); !a {
			t.Error("the locked topic was regenerated")
		}
	})
}

func TestNodeIDsResolveToTheirTopics(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	lecture := p.V.Lectures[0].V
	task := p.V.Assessment.V.SeminarTasks[0].V
	test := p.V.Assessment.V.Tests[0].V
	question := test.Questions[0].V
	outcome := p.V.Competencies[0].V.OutcomeIDs()[0]

	tests := []struct {
		name  string
		id    string
		parts func(target) bool
	}{
		{"a lecture means its topic's content", lecture.ID, func(t target) bool { return t[lecture.TopicID] == partContent }},
		{"a task means its topic's assessment", task.ID, func(t target) bool { return t[task.TopicID] == partAssessment }},
		{"a test means its module's assessment", test.ID, func(t target) bool {
			return len(t) > 0 && t[question.TopicIDs[0]] == partAssessment
		}},
		{"a question means its topic's assessment", question.ID, func(t target) bool { return t[question.TopicIDs[0]] == partAssessment }},
		{"an outcome means every topic that teaches it", outcome, func(t target) bool {
			for _, ref := range topicRefs(&p.V) {
				teaches := false
				for _, id := range ref.topic.OutcomeIDs {
					teaches = teaches || id == outcome
				}
				if teaches != (t[ref.topic.ID] == partAll) {
					return false
				}
			}
			return len(t) > 0
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets, err := resolveTargets(&p.V, []string{tt.id})
			if err != nil {
				t.Fatal(err)
			}
			if !tt.parts(targets) {
				t.Errorf("%s resolved to %v", tt.id, targets)
			}
		})
	}
}

func TestUnknownNodeIDsAreRefused(t *testing.T) {
	for _, id := range []string{"M99", "T9.9", "C9.K9", "L9.9", "ST999", "TEST99", "Q-nothing-1", "banana"} {
		t.Run(id, func(t *testing.T) {
			p := approvedProgram(t, 40, 0, "")
			generate(t, p)

			_, err := newService().Generate(context.Background(), p, []string{id})
			if !errors.Is(err, ErrUnknownNode) {
				t.Errorf("err = %v, want ErrUnknownNode", err)
			}
		})
	}
}

func TestAnEmptyNodeListMeansTheWholeProgram(t *testing.T) {
	p := completeProgram(t)

	if _, err := newService().Generate(context.Background(), p, []string{}); err != nil {
		t.Fatal(err)
	}
	for _, ref := range topicRefs(&p.V) {
		if a, _, _, _ := isMarked(p, ref.topic.ID); a {
			t.Errorf("%s was not regenerated", ref.topic.ID)
		}
	}
}

func TestTestsThisStepDidNotCreateAreLeftAlone(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	foreign := program.Test{ID: "TEST99", Title: "Итоговый тест", Purpose: "final",
		Questions: []program.Node[program.Question]{{V: program.Question{
			ID: "Q-final-1", Type: "short", Text: "Свой вопрос", Correct: []string{"x"},
			OutcomeIDs: []string{"C1.K1"}, Difficulty: "easy"}}}}
	p.V.Assessment.V.Tests = append(p.V.Assessment.V.Tests, program.Node[program.Test]{V: foreign})

	generate(t, p) // the whole program again

	found := false
	for _, test := range p.V.Assessment.V.Tests {
		if test.V.ID == "TEST99" {
			found = len(test.V.Questions) == 1 && test.V.Questions[0].V.Text == "Свой вопрос"
		}
	}
	if !found {
		t.Error("a test the generator did not create was changed or removed")
	}
}

func TestBreakingGeneratedContentIsCaughtByTheValidator(t *testing.T) {
	// Guards the end-to-end test above against passing for the wrong reason.
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	p.V.Lectures[0].V.Plan[0].V.Minutes += 5
	p.V.Assessment.V.Tests[0].V.Questions[0].V.Correct = []string{"zzz"}

	rules := ruleIDs(generate2(t, p))
	if !contains(rules, "LECTURE_MINUTES") || !contains(rules, "TEST_KEY") {
		t.Errorf("rules = %v, want LECTURE_MINUTES and TEST_KEY", rules)
	}
}

// Real data, not built by this package: the schema's own example program, with
// its lecture, tests, tasks and sources already in place.
func TestGenerationOverTheSchemaExample(t *testing.T) {
	data, err := os.ReadFile("../../../../schema/example_program.json")
	if err != nil {
		t.Fatalf("schema example not found: %v", err)
	}
	var p program.Program
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}

	issues := generate(t, &p)

	// The example's one known finding is its unverified third source.
	for _, c := range issues {
		if c.Severity == validate.SeverityError {
			t.Errorf("unexpected error: %+v", c)
		}
	}
	if len(issues) != 1 || issues[0].RuleID != "SOURCE_UNVERIFIED" {
		t.Errorf("issues = %+v, want only SOURCE_UNVERIFIED", issues)
	}
	if len(p.V.Sources) != 3 {
		t.Errorf("the example's %d sources must be kept, got %d", 3, len(p.V.Sources))
	}
}
