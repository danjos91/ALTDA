package content

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
)

// templateMark opens every text this generator writes, so template text is
// recognisable in the program and in the documents made from it.
const templateMark = "[Шаблон] "

// lectureSteps are the stages of a template lecture and their share of the time.
var lectureSteps = []struct {
	name  string
	share float64
}{
	{"Введение и цели занятия", 0.10},
	{"Основная часть", 0.55},
	{"Разбор примеров", 0.25},
	{"Итоги и вопросы", 0.10},
}

type templateGenerator struct{}

// NewTemplateGenerator returns the stand-in generator: it fills each topic with
// structurally correct content (hours that add up, answer keys that are valid,
// tasks with scored criteria) and no knowledge of the subject.
func NewTemplateGenerator() Generator {
	return templateGenerator{}
}

func (templateGenerator) Placeholder() bool { return true }

func (templateGenerator) Topic(_ context.Context, in TopicInput) (TopicContent, error) {
	return TopicContent{
		Activities: activitiesFor(in),
		Lecture:    lectureFor(in),
		Questions:  questionsFor(in),
		Task:       taskFor(in),
	}, nil
}

func activitiesFor(in TopicInput) []program.Activity {
	h := in.Topic.Hours
	title := in.Topic.Title
	candidates := []struct {
		kind  string
		hours float64
		text  string
	}{
		{"lecture", h.Lecture, "Лекция по теме «%s»"},
		{"practice", h.Practice, "Практическое занятие по теме «%s»"},
		{"lab", h.Lab, "Лабораторная работа по теме «%s»"},
		{"selfStudy", h.SelfStudy, "Самостоятельная работа по теме «%s»"},
		{"control", h.Assessment, "Контроль по теме «%s»"},
	}

	var activities []program.Activity
	for _, c := range candidates {
		if c.hours > 0 {
			activities = append(activities, program.Activity{
				Kind: c.kind, Hours: c.hours, Content: templateMark + fmt.Sprintf(c.text, title),
			})
		}
	}
	return activities
}

func lectureFor(in TopicInput) *program.Lecture {
	if in.Topic.Hours.Lecture <= 0 {
		return nil
	}

	// A topic's lecture minutes must equal its lecture hours x 45, rounded the
	// way the validator rounds (to even on .5).
	minutes := int(math.RoundToEven(in.Topic.Hours.Lecture * 45))
	weights := make([]float64, len(lectureSteps))
	for i, s := range lectureSteps {
		weights[i] = s.share
	}
	split := allocator.Distribute(minutes, weights)

	lecture := &program.Lecture{
		Title:      "Лекция: " + in.Topic.Title,
		Goal:       templateMark + "Сформулируйте цель лекции по теме «" + in.Topic.Title + "».",
		OutcomeIDs: outcomeIDs(in.Outcomes),
	}
	for i, s := range lectureSteps {
		if split[i] > 0 {
			lecture.Plan = append(lecture.Plan, program.Node[program.LectureStep]{
				V: program.LectureStep{Step: s.name, Minutes: split[i]},
			})
		}
	}
	return lecture
}

// questionsFor writes two questions per outcome of the topic, so every outcome
// has the two assessment tools the coverage rule asks for.
func questionsFor(in TopicInput) []program.Question {
	var questions []program.Question
	for i, outcome := range in.Outcomes {
		single := program.Question{
			Type: "single",
			Text: templateMark + fmt.Sprintf("Вопрос по теме «%s» (результат %s): выберите верный ответ.", in.Topic.Title, outcome.ID),
			Options: []program.Node[program.Option]{
				option("a", "Вариант ответа 1"), option("b", "Вариант ответа 2"), option("c", "Вариант ответа 3"),
			},
			Correct:     []string{[]string{"a", "b", "c"}[i%3]}, // rotate, so the key is not always the same
			Explanation: templateMark + "Замените пояснение.",
			OutcomeIDs:  []string{outcome.ID},
			Difficulty:  "medium",
			Points:      intPtr(1),
		}
		truefalse := program.Question{
			Type:        "truefalse",
			Text:        templateMark + fmt.Sprintf("Утверждение по теме «%s» (результат %s): верно ли оно?", in.Topic.Title, outcome.ID),
			Options:     []program.Node[program.Option]{option("t", "Верно"), option("f", "Неверно")},
			Correct:     []string{"t"},
			Explanation: templateMark + "Замените пояснение.",
			OutcomeIDs:  []string{outcome.ID},
			Difficulty:  "easy",
			Points:      intPtr(1),
		}
		questions = append(questions, single, truefalse)
	}
	return questions
}

func taskFor(in TopicInput) *program.SeminarTask {
	practical := in.Topic.Hours.Practice + in.Topic.Hours.Lab
	if practical <= 0 {
		return nil
	}

	outcomes := outcomeIDs(in.Outcomes)
	if !hasSkillOrAbility(outcomes) {
		// A task checks at least one skill (U) or ability (N).
		for _, o := range in.AllOutcomes {
			if isSkillOrAbility(o.ID) {
				outcomes = append(outcomes, o.ID)
				break
			}
		}
	}

	return &program.SeminarTask{
		Title:         "Практическое задание: " + in.Topic.Title,
		Format:        "индивидуально",
		Description:   templateMark + "Опишите условие практического задания по теме «" + in.Topic.Title + "».",
		Deliverable:   "Выполненное практическое задание",
		DurationHours: practical,
		OutcomeIDs:    outcomes,
		Criteria: []program.Node[program.Criterion]{
			{V: program.Criterion{Text: "Задание выполнено полностью, результат работает", MaxPoints: 3}},
			{V: program.Criterion{Text: "Решение оформлено и обосновано", MaxPoints: 2}},
		},
		MaxScore:      5,
		PassThreshold: intPtr(3),
	}
}

func option(id, text string) program.Node[program.Option] {
	return program.Node[program.Option]{V: program.Option{ID: id, Text: text}}
}

func intPtr(v int) *int { return &v }

func outcomeIDs(outcomes []OutcomeInfo) []string {
	ids := make([]string, len(outcomes))
	for i, o := range outcomes {
		ids[i] = o.ID
	}
	return ids
}

func isSkillOrAbility(id string) bool {
	return strings.Contains(id, ".U") || strings.Contains(id, ".N")
}

func hasSkillOrAbility(ids []string) bool {
	for _, id := range ids {
		if isSkillOrAbility(id) {
			return true
		}
	}
	return false
}
