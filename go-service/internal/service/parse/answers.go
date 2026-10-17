package parse

import (
	"strconv"
	"strings"
)

// applyAnswers writes the methodologist's answers into p. An explicit answer
// wins over whatever extract guessed from the text.
func applyAnswers(p *Params, clarifications []Clarification, gaps *[]string) {
	for _, c := range clarifications {
		if c.Answer == nil || strings.TrimSpace(*c.Answer) == "" {
			continue
		}
		answer := strings.TrimSpace(*c.Answer)

		if !applyAnswer(p, c.Param, answer, gaps) {
			*gaps = append(*gaps, "Ответ на параметр «"+c.Param+"» не удалось применить: "+answer)
		}
	}
}

func applyAnswer(p *Params, param, answer string, gaps *[]string) bool {
	switch param {
	case "programType":
		t := strings.ToLower(answer)
		switch {
		case answer == ProgramTypeDPPPK || strings.Contains(t, "дпп") || strings.Contains(t, "повыш"):
			p.ProgramType = strPtr(ProgramTypeDPPPK)
		case answer == ProgramTypeDOOP || strings.Contains(t, "дооп") || strings.Contains(t, "общеразв"):
			p.ProgramType = strPtr(ProgramTypeDOOP)
		default:
			return false
		}
	case "title":
		p.Title = strPtr(answer)
	case "domain":
		p.Domain = strPtr(answer)
	case "audience":
		p.Audience = strPtr(answer)
	case "entryLevel":
		p.EntryLevel = strPtr(answer)
	case "finalAttestationForm":
		p.FinalAttestationForm = strPtr(answer)
	case "studyForm":
		form := studyForm(strings.ToLower(answer))
		if form == nil {
			return false
		}
		p.StudyForm = form
	case "totalHours":
		v, err := strconv.ParseFloat(strings.Replace(answer, ",", ".", 1), 64)
		if err != nil {
			// "72 часа" and similar
			if m := hoursRe.FindStringSubmatch(strings.ToLower(answer) + " "); m != nil {
				v, err = strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
			}
		}
		if err != nil {
			return false
		}
		hours := roundHalf(v, gaps)
		if hours == nil {
			return false
		}
		p.TotalHours = hours
	default:
		return false
	}
	return true
}
