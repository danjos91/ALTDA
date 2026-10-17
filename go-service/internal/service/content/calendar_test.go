package content

import (
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

func TestCalendarSchedulesEverythingExceptSelfStudy(t *testing.T) {
	for _, params := range []string{``, `, "studyForm": "заочная"`, `, "sessionHours": 3`, `, "sessionHours": 2.5`} {
		p := approvedProgram(t, 72, 0, params)
		generate(t, p)

		sessions := p.V.Calendar.V.Sessions
		var scheduled float64
		for i, s := range sessions {
			scheduled += s.V.Hours
			if s.V.Number != i+1 {
				t.Errorf("%s: session %d has number %d", params, i+1, s.V.Number)
			}
			if s.V.Hours <= 0 {
				t.Errorf("%s: session %d has %v h", params, s.V.Number, s.V.Hours)
			}
		}

		totals := p.V.Curriculum.V.Totals
		if want := totals.Total - totals.SelfStudy; scheduled != want {
			t.Errorf("%s: sessions add up to %v h, want total - selfStudy = %v h", params, scheduled, want)
		}
	}
}

func TestSessionsNeverExceedTheirLength(t *testing.T) {
	for _, length := range []string{"4", "3", "2.5"} {
		p := approvedProgram(t, 72, 0, `, "sessionHours": `+length)
		generate(t, p)

		limit := 4.0
		switch length {
		case "3":
			limit = 3
		case "2.5":
			limit = 2.5
		}
		for _, s := range p.V.Calendar.V.Sessions {
			if s.V.Hours > limit {
				t.Errorf("sessionHours %s: session %d has %v h", length, s.V.Number, s.V.Hours)
			}
		}
	}
}

func TestCalendarWeeksFollowSessionsPerWeek(t *testing.T) {
	p := approvedProgram(t, 72, 0, `, "sessionsPerWeek": 2`)
	generate(t, p)

	for _, s := range p.V.Calendar.V.Sessions {
		if want := (s.V.Number + 1) / 2; s.V.Week == nil || *s.V.Week != want {
			t.Errorf("session %d: week = %v, want %d", s.V.Number, s.V.Week, want)
		}
	}
}

func TestTheFinalAttestationIsScheduledAndFlagged(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	var hours float64
	for _, s := range p.V.Calendar.V.Sessions {
		if s.V.FinalAttestation != nil && *s.V.FinalAttestation {
			hours += s.V.Hours
		}
	}
	if hours == 0 {
		t.Error("no session is flagged as holding the final attestation")
	}
	last := p.V.Calendar.V.Sessions[len(p.V.Calendar.V.Sessions)-1].V
	if last.FinalAttestation == nil || !*last.FinalAttestation {
		t.Error("the last session must include the final attestation")
	}
}

func TestLecturesKnowTheirSessions(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	if len(p.V.Lectures) == 0 {
		t.Fatal("no lectures")
	}
	for _, l := range p.V.Lectures {
		if len(l.V.SessionNumbers) == 0 {
			t.Errorf("lecture %s has no session numbers", l.V.ID)
		}
		for _, n := range l.V.SessionNumbers {
			if n < 1 || n > len(p.V.Calendar.V.Sessions) {
				t.Errorf("lecture %s points at session %d", l.V.ID, n)
			}
		}
	}
}

func TestAnInternshipHasOnlyTheAttestationOnTheCalendar(t *testing.T) {
	p := approvedProgram(t, 72, 0, `, "isInternship": true`)
	generate(t, p)

	var scheduled float64
	for _, s := range p.V.Calendar.V.Sessions {
		scheduled += s.V.Hours
		if len(s.V.TopicIDs) != 0 {
			t.Errorf("session %d lists topics %v, but everything is self-study", s.V.Number, s.V.TopicIDs)
		}
	}
	if scheduled != p.V.FinalAttestation.V.Hours.Total {
		t.Errorf("scheduled %v h, want only the attestation %v h", scheduled, p.V.FinalAttestation.V.Hours.Total)
	}
	if len(p.V.Lectures) != 0 {
		t.Errorf("an internship has %d lectures", len(p.V.Lectures))
	}
}

func TestTemplateSectionsReflectTheState(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	status := map[string]string{}
	for _, s := range p.V.TemplateSections {
		status[s.V.Key] = s.V.Status
	}
	want := map[string]string{
		"title_page": "placeholder", "explanatory_note": "filled", "planned_results": "filled",
		"curriculum": "filled", "calendar": "filled",
		"lectures": "placeholder", "assessment_materials": "placeholder",
		"sources": "missing", "normative_basis": "missing",
	}
	for key, w := range want {
		if status[key] != w {
			t.Errorf("section %s = %q, want %q", key, status[key], w)
		}
	}
}

func TestAnExistingSectionListIsRespected(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	p.V.TemplateSections = []program.Node[program.TemplateSection]{
		{V: program.TemplateSection{Key: "annex_a", Title: "Приложение А", Status: "filled"}},
		{V: program.TemplateSection{Key: "curriculum", Title: "Учебный план (по образцу АЦР)", Status: "missing"}},
	}

	generate(t, p)

	byKey := map[string]program.TemplateSection{}
	for _, s := range p.V.TemplateSections {
		byKey[s.V.Key] = s.V
	}
	if byKey["annex_a"].Title != "Приложение А" || byKey["annex_a"].Status != "filled" {
		t.Errorf("a section the step does not know was changed: %+v", byKey["annex_a"])
	}
	if byKey["curriculum"].Title != "Учебный план (по образцу АЦР)" || byKey["curriculum"].Status != "filled" {
		t.Errorf("a known section should keep its title and get its status updated: %+v", byKey["curriculum"])
	}
	if p.V.TemplateSections[0].V.Key != "annex_a" {
		t.Error("the order of an existing list must be kept")
	}
}
