package parse

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func parseText(t *testing.T, text string, answers ...Clarification) *Result {
	t.Helper()
	res, err := NewService().Parse(context.Background(), Input{RawText: text, Clarifications: answers})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func answer(param, value string) Clarification {
	return Clarification{Param: param, Answer: &value}
}

func TestExtractsExplicitValues(t *testing.T) {
	res := parseText(t, "ДПП ПК Анализ данных на Python, 72 ч, очно-заочная форма, дистанционно, защита проекта, практики не менее 60%")
	p := res.Params

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"programType", deref(p.ProgramType), ProgramTypeDPPPK},
		{"totalHours", derefF(p.TotalHours), 72.0},
		{"studyForm", deref(p.StudyForm), "очно-заочная"},
		{"distanceTech", *p.DistanceTech, true},
		{"finalAttestationForm", deref(p.FinalAttestationForm), "защита проекта"},
		{"minPracticeShare", derefF(p.MinPracticeShare), 0.6},
		{"title", deref(p.Title), "Анализ данных на Python"},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestTotalHoursTakesLargestFigure(t *testing.T) {
	res := parseText(t, "Курс на 40 часов, занятия по 4 часа")
	if got := derefF(res.Params.TotalHours); got != 40 {
		t.Errorf("totalHours = %v, want 40", got)
	}
}

func TestHoursDoNotMatchInsideOtherWords(t *testing.T) {
	// "72 человека" must not be read as 72 hours.
	res := parseText(t, "Группа 72 человека")
	if res.Params.TotalHours != nil {
		t.Errorf("totalHours = %v, want nil", *res.Params.TotalHours)
	}
}

func TestProgramTypeDOOP(t *testing.T) {
	res := parseText(t, "Дополнительная общеразвивающая программа по робототехнике, 24 часа")
	if got := deref(res.Params.ProgramType); got != ProgramTypeDOOP {
		t.Errorf("programType = %q, want %q", got, ProgramTypeDOOP)
	}
}

func TestBothProgramTypesLeavesItMissing(t *testing.T) {
	res := parseText(t, "ДПП или ДООП по Python, 32 ч")
	if res.Params.ProgramType != nil {
		t.Error("programType must be nil when both types are mentioned")
	}
	if !contains(res.Missing, "programType") {
		t.Errorf("missing = %v, want programType in it", res.Missing)
	}
}

func TestMissingListsRequiredParamsInOrder(t *testing.T) {
	res := parseText(t, "Python")
	want := []string{"programType", "domain", "totalHours", "audience", "entryLevel"}
	if !reflect.DeepEqual(res.Missing, want) {
		t.Errorf("missing = %v, want %v", res.Missing, want)
	}
	if len(res.Clarifications) != len(want) {
		t.Errorf("clarifications = %d, want one question per missing param", len(res.Clarifications))
	}
}

func TestAnswersFillMissingAndOverrideText(t *testing.T) {
	res := parseText(t, "ДПП ПК Анализ данных, 72 ч",
		answer("domain", "Анализ данных"),
		answer("audience", "Аналитики"),
		answer("entryLevel", "Базовый"),
		answer("totalHours", "80"),
	)

	if len(res.Missing) != 0 {
		t.Errorf("missing = %v, want none", res.Missing)
	}
	if got := derefF(res.Params.TotalHours); got != 80 {
		t.Errorf("totalHours = %v, want 80 (answer beats text)", got)
	}
}

func TestDefaultsAreReportedAsGaps(t *testing.T) {
	res := parseText(t, "ДПП ПК Python, 72 ч")

	if deref(res.Params.StudyForm) != "очная" {
		t.Errorf("studyForm = %q, want default очная", deref(res.Params.StudyForm))
	}
	if got := *res.Params.DurationWeeks; got != 9 { // ceil(72 / (4*2))
		t.Errorf("durationWeeks = %d, want 9", got)
	}
	if len(res.GapsReport) == 0 {
		t.Error("assumed defaults must be listed in gapsReport")
	}
}

func TestHoursAreRoundedToHalf(t *testing.T) {
	res := parseText(t, "ДПП Python 72,3 ч")
	if got := derefF(res.Params.TotalHours); got != 72.5 {
		t.Errorf("totalHours = %v, want 72.5", got)
	}
}

func TestUnappliedAnswerIsReported(t *testing.T) {
	res := parseText(t, "ДПП Python 72 ч", answer("totalHours", "много"))

	found := false
	for _, gap := range res.GapsReport {
		if strings.Contains(gap, "totalHours") {
			found = true
		}
	}
	if !found {
		t.Errorf("gapsReport = %v, want a note about totalHours", res.GapsReport)
	}
}

func TestParamsNeverNilSlices(t *testing.T) {
	res := parseText(t, "x")
	if res.Params.DeliverablesRequested == nil || res.Params.Constraints == nil ||
		res.Missing == nil || res.Clarifications == nil || res.GapsReport == nil {
		t.Error("slices must be non-nil so they encode as [] and not null")
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func derefF(f *float64) float64 {
	if f == nil {
		return -1
	}
	return *f
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
