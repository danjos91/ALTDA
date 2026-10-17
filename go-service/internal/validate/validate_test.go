package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/danjos91/ALTDA/go-service/internal/program"
)

func TestValidateGoodFixture(t *testing.T) {
	prog := loadFixture(t, "example_program.json")
	checks := ValidateProgram(prog)
	if len(checks) == 0 {
		t.Fatal("expected checks on good fixture")
	}
	ruleIDs := map[string]bool{}
	for _, c := range checks {
		ruleIDs[c.RuleID] = true
	}
	for _, want := range []string{
		"HOURS_CONTACT", "HOURS_ROW_SUM", "HOURS_TOPICS_EQ_MODULE",
		"HOURS_TOTALS_FIELD", "HOURS_EQ_REQUESTED", "COVERAGE_OUTCOME_TAUGHT",
		"SOURCE_UNVERIFIED",
	} {
		if !ruleIDs[want] {
			t.Fatalf("missing ruleId %s in checks", want)
		}
	}
	_, issues := BuildReport(checks, 0)
	if issues == nil {
		t.Fatal("issues must be non-nil")
	}
	// Good fixture should have no HOURS_EQ_REQUESTED failure.
	for _, c := range checks {
		if c.RuleID == "HOURS_EQ_REQUESTED" && !c.Passed {
			t.Fatalf("good fixture failed HOURS_EQ_REQUESTED: %s", c.Message)
		}
	}
}

func TestValidateBadHours(t *testing.T) {
	prog := loadFixture(t, "example_program.json")
	program.EnsureObject(prog, "header")["totalHours"] = 999.0
	checks := ValidateProgram(prog)
	found := false
	for _, c := range checks {
		if c.RuleID == "HOURS_EQ_REQUESTED" && !c.Passed {
			found = true
			if c.Path != "/header/totalHours" {
				t.Fatalf("path=%q want /header/totalHours", c.Path)
			}
		}
	}
	if !found {
		t.Fatal("expected HOURS_EQ_REQUESTED failure")
	}
	_, issues := BuildReport(checks, 0)
	flat := false
	for _, i := range issues {
		if i.RuleID == "HOURS_EQ_REQUESTED" {
			flat = true
		}
	}
	if !flat {
		t.Fatal("failed checks must appear in flat issues")
	}
}

func TestValidateSkeletonResponseRuleIDs(t *testing.T) {
	resp := loadFixture(t, "example_skeleton_response.json")
	checks := ValidateSkeletonResponse(resp)
	ruleIDs := map[string]bool{}
	for _, c := range checks {
		ruleIDs[c.RuleID] = true
	}
	for _, want := range []string{
		"SKELETON_MODULE_WEIGHTS", "SKELETON_TOPIC_WEIGHTS",
		"SKELETON_DANGLING_OUTCOME", "COVERAGE_OUTCOME_TAUGHT", "ORDER_PREREQUISITES",
	} {
		if !ruleIDs[want] {
			t.Fatalf("missing skeleton ruleId %s", want)
		}
	}
}

func TestRowSumIdentities(t *testing.T) {
	var checks []Check
	h := program.Hours{Lecture: 2, Practice: 3, Lab: 1, SelfStudy: 1, Assessment: 0.5, Contact: 6, Total: 7.5}
	checkRow(&checks, h, "/x", "X")
	for _, c := range checks {
		if !c.Passed {
			t.Fatalf("good row failed %s: %s", c.RuleID, c.Message)
		}
	}
	bad := h
	bad.Contact = 5
	checks = nil
	checkRow(&checks, bad, "/x", "X")
	failed := false
	for _, c := range checks {
		if c.RuleID == "HOURS_CONTACT" && !c.Passed {
			failed = true
		}
	}
	if !failed {
		t.Fatal("expected HOURS_CONTACT failure")
	}
}

func loadFixture(t *testing.T, name string) program.Object {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var o program.Object
	if err := json.Unmarshal(b, &o); err != nil {
		t.Fatal(err)
	}
	return o
}
