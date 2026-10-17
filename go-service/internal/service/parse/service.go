package parse

import (
	"context"
	"math"
	"strings"
)

const (
	defaultStudyForm       = "очная"
	defaultSessionHours    = 4.0
	defaultSessionsPerWeek = 2.0
)

type service struct{}

func NewService() Service {
	return &service{}
}

func (s *service) Parse(_ context.Context, in Input) (*Result, error) {
	gaps := []string{}

	params := extract(in.RawText, &gaps)
	applyAnswers(&params, in.Clarifications, &gaps)
	fillDefaults(&params, &gaps)

	missing := missingParams(params)

	return &Result{
		Params:         params,
		Missing:        missing,
		Clarifications: buildClarifications(in.Clarifications, missing),
		GapsReport:     gaps,
	}, nil
}

// fillDefaults sets the values the schema allows us to assume and records
// each assumption so the methodologist can see it.
func fillDefaults(p *Params, gaps *[]string) {
	if p.StudyForm == nil {
		p.StudyForm = strPtr(defaultStudyForm)
		*gaps = append(*gaps, "Форма обучения не указана; принята «"+defaultStudyForm+"».")
	}
	if p.SessionHours == nil {
		hours := defaultSessionHours
		p.SessionHours = &hours
	}
	if p.SessionsPerWeek == nil {
		perWeek := defaultSessionsPerWeek
		p.SessionsPerWeek = &perWeek
	}
	if p.DurationWeeks == nil && p.TotalHours != nil {
		weeks := int(math.Ceil(*p.TotalHours / (*p.SessionHours * *p.SessionsPerWeek)))
		if weeks < 1 {
			weeks = 1
		}
		p.DurationWeeks = &weeks
		*gaps = append(*gaps, "Срок обучения рассчитан из объёма часов при 4 часах в занятие и 2 занятиях в неделю.")
	}
	if p.TotalHours != nil && (*p.TotalHours < 16 || *p.TotalHours > 500) {
		*gaps = append(*gaps, "Объём программы вне диапазона 16–500 часов.")
	}
}

func missingParams(p Params) []string {
	known := map[string]bool{
		"programType": p.ProgramType != nil,
		"title":       p.Title != nil,
		"domain":      p.Domain != nil,
		"totalHours":  p.TotalHours != nil,
		"audience":    p.Audience != nil,
		"entryLevel":  p.EntryLevel != nil,
	}

	missing := []string{}
	for _, param := range requiredParams {
		if !known[param] {
			missing = append(missing, param)
		}
	}
	return missing
}

// buildClarifications keeps the questions already asked (with their answers)
// and adds one for every missing param that has none yet.
func buildClarifications(asked []Clarification, missing []string) []Clarification {
	out := []Clarification{}
	seen := map[string]int{}

	for _, c := range asked {
		question, known := questions[c.Param]
		if !known {
			continue
		}
		if strings.TrimSpace(c.Question) == "" || c.Question == "auto" {
			c.Question = question
		}
		if i, dup := seen[c.Param]; dup {
			if c.Answer != nil && strings.TrimSpace(*c.Answer) != "" {
				out[i] = c
			}
			continue
		}
		seen[c.Param] = len(out)
		out = append(out, c)
	}

	for _, param := range missing {
		if _, ok := seen[param]; ok {
			continue
		}
		out = append(out, Clarification{Param: param, Question: questions[param]})
	}
	return out
}
