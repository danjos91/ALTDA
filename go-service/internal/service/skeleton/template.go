package skeleton

import (
	"context"
	"fmt"
	"math"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

const templateNote = "Шаблонный вариант: структура построена без анализа предметной области. " +
	"Названия модулей и тем, а также формулировки результатов нужно заменить."

// templateDrafter is a stand-in for the LLM: it proposes three structurally
// different skeletons from the request alone. It invents no content about the
// subject and no professional standards (profStandardLinks stay empty).
type templateDrafter struct{}

func NewTemplateDrafter() Drafter {
	return templateDrafter{}
}

// shape describes one variant; the builder below does the rest.
type shape struct {
	key          string
	name         string
	stages       []string // module titles, in order
	introFirst   bool
	practiceHint *float64
	kinds        []string
	attestation  string
	pros, cons   []string
}

func hint(v float64) *float64 { return &v }

var shapes = []shape{
	{
		key:  "A",
		name: "Линейный курс: от основ к практике",
		stages: []string{
			"Введение и основные понятия", "Базовые инструменты", "Методы и приёмы работы",
			"Прикладные задачи", "Углублённые темы", "Итоговый практикум",
		},
		introFirst:  true,
		kinds:       []string{"lecture", "practice"},
		attestation: "зачёт",
		pros:        []string{"Понятная последовательность для слушателей без опыта"},
		cons:        []string{"Практическая часть смещена к концу курса"},
	},
	{
		key:  "B",
		name: "Проектный курс: практика с первого модуля",
		stages: []string{
			"Введение и постановка проекта", "Основы для проекта", "Разработка проекта: этап 1",
			"Разработка проекта: этап 2", "Доработка и защита проекта",
		},
		introFirst:   true,
		practiceHint: hint(0.8),
		kinds:        []string{"practice", "lab"},
		attestation:  "защита проекта",
		pros:         []string{"Слушатели получают готовый результат"},
		cons:         []string{"Теории меньше, нужны слушатели с базовой подготовкой"},
	},
	{
		key:  "C",
		name: "Лекционно-семинарский курс",
		stages: []string{
			"Теоретические основы", "Семинар: инструменты", "Семинар: методы", "Семинар: прикладные задачи",
		},
		practiceHint: hint(0.4),
		kinds:        []string{"lecture", "practice"},
		attestation:  "тестирование",
		pros:         []string{"Систематическое изложение теории"},
		cons:         []string{"Меньше времени на самостоятельную практику"},
	},
}

func (templateDrafter) Draft(_ context.Context, req Request) ([]skeleton.Draft, error) {
	drafts := make([]skeleton.Draft, 0, len(shapes))
	for _, s := range shapes {
		drafts = append(drafts, build(s, req))
	}
	return drafts, nil
}

func build(s shape, req Request) skeleton.Draft {
	subject := subjectOf(req)
	competencies := templateCompetencies(subject)
	outcomes := outcomeIDs(competencies)

	topicCount := topicCountFor(req.TotalHours)
	moduleCount := min(max(topicCount/4, 2), len(s.stages))
	perModule := split(topicCount, moduleCount)

	kinds := kindsFor(s, req)
	topicHint := s.practiceHint
	if topicHint != nil && req.MinPracticeShare != nil && *req.MinPracticeShare > *topicHint {
		topicHint = req.MinPracticeShare // do not propose a split the request rules out
	}

	draft := skeleton.Draft{
		Key:          s.key,
		Rationale:    fmt.Sprintf("%s. %s", s.name, templateNote),
		Pros:         s.pros,
		Cons:         append(append([]string{}, s.cons...), "Шаблонная структура, не учитывает специфику предмета"),
		Goal:         fmt.Sprintf("Сформировать у слушателей знания, умения и навыки в области «%s».", subject),
		Competencies: competencies,
		FinalAttestation: skeleton.FinalAttestation{
			Form:        attestationFor(s, req),
			Description: fmt.Sprintf("Итоговая аттестация по программе «%s».", req.Title),
			OutcomeIDs:  []string{outcomes[0], outcomes[len(outcomes)-1]},
		},
	}

	global := 0
	for mi := 0; mi < moduleCount; mi++ {
		module := skeleton.Module{
			ID:      fmt.Sprintf("M%d", mi+1),
			Title:   s.stages[mi],
			IsIntro: s.introFirst && mi == 0,
			Weight:  roundTo(float64(perModule[mi])/float64(topicCount), 4),
		}

		for ti := 0; ti < perModule[mi]; ti++ {
			topic := skeleton.Topic{
				ID:                fmt.Sprintf("T%d.%d", mi+1, ti+1),
				Title:             topicTitle(s.stages[mi], ti, perModule[mi]),
				Weight:            roundTo(1/float64(perModule[mi]), 4),
				Kinds:             kinds,
				PracticeShareHint: topicHint,
				// Spread the outcomes over the program in order, so every one is taught.
				OutcomeIDs:    []string{outcomes[global*len(outcomes)/topicCount]},
				Prerequisites: []string{},
			}
			if ti > 0 {
				topic.Prerequisites = []string{fmt.Sprintf("T%d.%d", mi+1, ti)}
			}
			module.Topics = append(module.Topics, topic)
			global++
		}
		draft.Modules = append(draft.Modules, module)
	}

	fixWeights(&draft)
	return draft
}

// topicCountFor sizes the program: roughly one topic per three hours,
// between 4 and 40, and never more than the hours can hold at one hour each
// plus the attestation.
func topicCountFor(hours float64) int {
	count := int(math.Round(hours / 3))
	count = min(max(count, 4), 40)
	return max(min(count, int(hours)-1), 1)
}

// split divides total into n parts that differ by at most one, larger first.
func split(total, n int) []int {
	parts := make([]int, n)
	for i := range parts {
		parts[i] = total / n
		if i < total%n {
			parts[i]++
		}
	}
	return parts
}

func kindsFor(s shape, req Request) []string {
	if req.IsInternship {
		return []string{"selfStudy"}
	}
	kinds := append([]string{}, s.kinds...)
	if req.StudyForm != "очная" {
		kinds = append(kinds, "selfStudy")
	}
	return kinds
}

var validAttestations = map[string]bool{
	"зачёт": true, "экзамен": true, "тестирование": true, "защита проекта": true,
	"защита итоговой работы": true, "демонстрационный экзамен": true,
}

func attestationFor(s shape, req Request) string {
	if validAttestations[req.FinalAttestationForm] {
		return req.FinalAttestationForm
	}
	return s.attestation
}

func subjectOf(req Request) string {
	if req.Domain != "" {
		return req.Domain
	}
	return req.Title
}

func topicTitle(stage string, index, count int) string {
	if count == 1 {
		return stage
	}
	return fmt.Sprintf("%s: часть %d", stage, index+1)
}

func templateCompetencies(subject string) []skeleton.Competency {
	return []skeleton.Competency{
		{
			ID:    "C1",
			Title: fmt.Sprintf("Теоретические основы и инструменты области «%s»", subject),
			Kind:  "новая",
			Knowledge: []skeleton.Outcome{
				{ID: "C1.K1", Text: fmt.Sprintf("Знать основные понятия, методы и инструменты области «%s»", subject)},
			},
			Skills: []skeleton.Outcome{
				{ID: "C1.U1", Text: fmt.Sprintf("Уметь применять базовые инструменты области «%s» для решения типовых задач", subject)},
			},
		},
		{
			ID:    "C2",
			Title: fmt.Sprintf("Практическое применение в области «%s»", subject),
			Kind:  "новая",
			Skills: []skeleton.Outcome{
				{ID: "C2.U1", Text: fmt.Sprintf("Уметь самостоятельно решать прикладные задачи в области «%s»", subject)},
			},
			Abilities: []skeleton.Outcome{
				{ID: "C2.N1", Text: fmt.Sprintf("Владеть навыками выполнения практических работ в области «%s»", subject)},
			},
		},
	}
}

func outcomeIDs(competencies []skeleton.Competency) []string {
	return skeleton.Draft{Competencies: competencies}.OutcomeIDs()
}

func roundTo(v float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(v*scale) / scale
}

// fixWeights makes each group of weights add up to exactly 1 after rounding,
// by giving the rounding difference to the last item.
func fixWeights(d *skeleton.Draft) {
	var sum float64
	for i := range d.Modules {
		sum += d.Modules[i].Weight
	}
	last := &d.Modules[len(d.Modules)-1]
	last.Weight = roundTo(last.Weight+(1-sum), 4)

	for mi := range d.Modules {
		topics := d.Modules[mi].Topics
		var topicSum float64
		for _, t := range topics {
			topicSum += t.Weight
		}
		topics[len(topics)-1].Weight = roundTo(topics[len(topics)-1].Weight+(1-topicSum), 4)
	}
}
