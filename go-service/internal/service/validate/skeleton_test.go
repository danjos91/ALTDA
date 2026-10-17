package validate

import (
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

func goodDraft() skeleton.Draft {
	return skeleton.Draft{
		Key: "A",
		Competencies: []skeleton.Competency{{
			ID:        "C1",
			Knowledge: []skeleton.Outcome{{ID: "C1.K1", Text: "знать"}},
			Skills:    []skeleton.Outcome{{ID: "C1.U1", Text: "уметь"}},
		}},
		Modules: []skeleton.Module{
			{ID: "M1", Weight: 0.5, Topics: []skeleton.Topic{
				{ID: "T1.1", Weight: 0.5, OutcomeIDs: []string{"C1.K1"}},
				{ID: "T1.2", Weight: 0.5, OutcomeIDs: []string{"C1.U1"}, Prerequisites: []string{"T1.1"}},
			}},
			{ID: "M2", Weight: 0.5, Topics: []skeleton.Topic{
				{ID: "T2.1", Weight: 1, OutcomeIDs: []string{"C1.K1"}, Prerequisites: []string{"T1.2"}},
			}},
		},
	}
}

func failedRules(checks []Check) map[string]bool {
	rules := map[string]bool{}
	for _, c := range checks {
		if !c.Passed {
			rules[c.RuleID] = true
		}
	}
	return rules
}

func TestGoodDraftPassesEveryRule(t *testing.T) {
	checks := CheckDraft(goodDraft())

	if len(checks) == 0 {
		t.Fatal("no checks ran")
	}
	if failed := failedRules(checks); len(failed) != 0 {
		t.Errorf("failed rules: %v", failed)
	}
	if len(Failed(checks)) != 0 {
		t.Error("Failed() must be empty for a good draft")
	}
}

func TestEachDraftRuleCatchesItsOwnBreakage(t *testing.T) {
	tests := []struct {
		ruleID   string
		breakage func(*skeleton.Draft)
	}{
		{"SKELETON_MODULE_WEIGHTS", func(d *skeleton.Draft) { d.Modules[0].Weight = 0.8 }},
		{"SKELETON_TOPIC_WEIGHTS", func(d *skeleton.Draft) { d.Modules[0].Topics[0].Weight = 0.9 }},
		{"SKELETON_DANGLING_OUTCOME", func(d *skeleton.Draft) { d.Modules[0].Topics[0].OutcomeIDs = []string{"C9.K9"} }},
		{"COVERAGE_OUTCOME_TAUGHT", func(d *skeleton.Draft) {
			d.Competencies[0].Abilities = []skeleton.Outcome{{ID: "C1.N1", Text: "владеть"}}
		}},
		{"ORDER_PREREQUISITES", func(d *skeleton.Draft) { d.Modules[0].Topics[0].Prerequisites = []string{"T2.1"} }},
		{"ORDER_PREREQUISITES", func(d *skeleton.Draft) { d.Modules[0].Topics[0].Prerequisites = []string{"T9.9"} }},
	}

	for _, tt := range tests {
		t.Run(tt.ruleID, func(t *testing.T) {
			d := goodDraft()
			tt.breakage(&d)

			checks := CheckDraft(d)
			if !failedRules(checks)[tt.ruleID] {
				t.Fatalf("%s did not fail", tt.ruleID)
			}
			if len(Failed(checks)) == 0 {
				t.Error("an error-severity failure must show up in Failed()")
			}
		})
	}
}

func TestWeightsWithinToleranceAreAccepted(t *testing.T) {
	d := goodDraft()
	d.Modules[0].Weight = 0.5 - 0.005 // 0.995 in total: inside the 0.01 tolerance

	if failedRules(CheckDraft(d))["SKELETON_MODULE_WEIGHTS"] {
		t.Error("a sum within 0.01 of 1 must pass")
	}
}
