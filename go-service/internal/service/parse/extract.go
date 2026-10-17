package parse

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// RE2 has no Unicode-aware \b or \w, so word edges are spelled with \p{L}.
var (
	hoursRe       = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(?:часов|часа|час|ч)(?:[^\p{L}]|$)`)
	weeksRe       = regexp.MustCompile(`(\d+)\s*(?:недел\p{L}*|нед)(?:[^\p{L}]|$)`)
	shareAfter    = regexp.MustCompile(`практик\p{L}*[^.%]{0,40}?(\d{1,3})\s*%`)
	shareFirst    = regexp.MustCompile(`(\d{1,3})\s*%[^.]{0,20}практик`)
	clauseSplitRe = regexp.MustCompile(`[,;.\n]`)
	audienceRe    = regexp.MustCompile(`(?i)для\s+([^.,;\n]{3,80})`)
)

// titleNoise are words dropped when guessing the title from the request.
var titleNoise = map[string]bool{
	"дпп": true, "пк": true, "дооп": true, "программа": true, "программу": true,
	"курс": true, "повышения": true, "квалификации": true,
}

// extract reads what the text states explicitly. Anything it cannot find
// stays nil.
func extract(raw string, gaps *[]string) Params {
	text := strings.ToLower(raw)
	p := Params{
		DeliverablesRequested: []string{},
		Constraints:           []string{},
		IsInternship:          strings.Contains(text, "стажиров"),
	}

	p.ProgramType = programType(text, gaps)
	p.TotalHours = totalHours(text, gaps)
	p.StudyForm = studyForm(text)
	p.DistanceTech = boolPtr(containsAny(text, "дистанцион", "онлайн", "электронн"))
	p.FinalAttestationForm = attestationForm(text)
	p.EntryLevel = entryLevel(text)
	p.MinPracticeShare = practiceShare(text)

	if m := weeksRe.FindStringSubmatch(text); m != nil {
		if weeks, err := strconv.Atoi(m[1]); err == nil && weeks > 0 {
			p.DurationWeeks = &weeks
		}
	}
	if m := audienceRe.FindStringSubmatch(raw); m != nil {
		p.Audience = strPtr(strings.TrimSpace(m[1]))
	}
	if containsAny(text, "тест") {
		p.DeliverablesRequested = append(p.DeliverablesRequested, "тесты")
	}
	if containsAny(text, "презентац", "слайд") {
		p.DeliverablesRequested = append(p.DeliverablesRequested, "презентации")
	}

	if title := guessTitle(raw); title != "" {
		p.Title = &title
		*gaps = append(*gaps, "Название взято из текста запроса автоматически — проверьте его.")
	}
	return p
}

func programType(text string, gaps *[]string) *string {
	dpp := strings.Contains(text, "дпп") || (strings.Contains(text, "повышен") && strings.Contains(text, "квалификац"))
	doop := strings.Contains(text, "дооп") || strings.Contains(text, "общеразвивающ")

	switch {
	case dpp && doop:
		*gaps = append(*gaps, "В запросе упомянуты оба типа программ (ДПП и ДООП); тип нужно уточнить.")
		return nil
	case dpp:
		return strPtr(ProgramTypeDPPPK)
	case doop:
		return strPtr(ProgramTypeDOOP)
	}
	return nil
}

// totalHours takes the largest hour figure in the text: smaller ones are
// usually session lengths ("по 4 часа").
func totalHours(text string, gaps *[]string) *float64 {
	var best float64
	distinct := map[float64]bool{}
	for _, m := range hoursRe.FindAllStringSubmatch(text, -1) {
		v, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
		if err != nil || v <= 0 {
			continue
		}
		distinct[v] = true
		best = math.Max(best, v)
	}
	if best == 0 {
		return nil
	}
	if len(distinct) > 1 {
		*gaps = append(*gaps, "В запросе несколько значений часов; общим объёмом принято наибольшее.")
	}
	return roundHalf(best, gaps)
}

func studyForm(text string) *string {
	switch {
	case strings.Contains(text, "очно-заочн"):
		return strPtr("очно-заочная")
	case strings.Contains(text, "заочн"):
		return strPtr("заочная")
	case strings.Contains(text, "очн"):
		return strPtr("очная")
	}
	return nil
}

func attestationForm(text string) *string {
	switch {
	case strings.Contains(text, "защит") && strings.Contains(text, "проект"):
		return strPtr("защита проекта")
	case strings.Contains(text, "экзамен"):
		return strPtr("экзамен")
	case strings.Contains(text, "зачёт") || strings.Contains(text, "зачет"):
		return strPtr("зачёт")
	case strings.Contains(text, "тестирован"):
		return strPtr("тестирование")
	}
	return nil
}

func entryLevel(text string) *string {
	switch {
	case containsAny(text, "с нуля", "начинающ", "новичк", "базов"):
		return strPtr("Базовый")
	case containsAny(text, "продвинут", "опытн"):
		return strPtr("Продвинутый")
	case strings.Contains(text, "средн"):
		return strPtr("Средний")
	}
	return nil
}

func practiceShare(text string) *float64 {
	m := shareAfter.FindStringSubmatch(text)
	if m == nil {
		m = shareFirst.FindStringSubmatch(text)
	}
	if m == nil {
		return nil
	}
	percent, err := strconv.Atoi(m[1])
	if err != nil || percent > 100 {
		return nil
	}
	share := float64(percent) / 100
	return &share
}

// guessTitle takes the first clause of the request that is not just program-type noise.
func guessTitle(raw string) string {
	cleaned := hoursRe.ReplaceAllString(raw, " ")

	for _, clause := range clauseSplitRe.Split(cleaned, -1) {
		var kept []string
		for _, word := range strings.Fields(clause) {
			if titleNoise[strings.ToLower(strings.Trim(word, ",.;:-—()«»\""))] {
				continue
			}
			kept = append(kept, word)
		}

		title := strings.Trim(strings.Join(kept, " "), " -—")
		if utf8.RuneCountInString(title) < 3 {
			continue
		}
		if runes := []rune(title); len(runes) > 120 {
			title = string(runes[:120])
		}
		return title
	}
	return ""
}

// roundHalf rounds to the 0.5 step the schema requires.
func roundHalf(v float64, gaps *[]string) *float64 {
	rounded := math.Round(v*2) / 2
	if rounded != v {
		*gaps = append(*gaps, "Количество часов округлено до 0,5.")
	}
	if rounded <= 0 {
		return nil
	}
	return &rounded
}

func containsAny(text string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}

func strPtr(s string) *string { return &s }

func boolPtr(b bool) *bool { return &b }
