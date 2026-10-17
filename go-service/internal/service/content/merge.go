package content

import (
	"fmt"
	"strings"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

// moduleNumber and topicNumbers read the digits of an ID: "M2" is 2, "T2.3" is (2, 3).
func moduleNumber(moduleID string) int {
	var m int
	fmt.Sscanf(moduleID, "M%d", &m)
	return m
}

func topicNumbers(topicID string) (module, topic int) {
	fmt.Sscanf(topicID, "T%d.%d", &module, &topic)
	return module, topic
}

func lectureID(topicID string) string { return "L" + strings.TrimPrefix(topicID, "T") }

func taskID(topicID string) string {
	m, t := topicNumbers(topicID)
	return fmt.Sprintf("ST%d", m*100+t)
}

func testID(moduleID string) string { return fmt.Sprintf("TEST%d", moduleNumber(moduleID)) }

func questionID(topicID string, i int) string {
	m, t := topicNumbers(topicID)
	return fmt.Sprintf("Q-M%d-T%d-%d", m, t, i+1)
}

// merge writes the generated content of the targeted topics into the program
// and keeps what exists for the others.
func merge(root *program.Root, refs []topicRef, generated map[string]TopicContent, targets target) {
	for _, ref := range refs {
		if targets[ref.topic.ID]&partContent == 0 {
			continue
		}
		content := generated[ref.topic.ID]
		ref.topic.Activities = nil
		for _, a := range content.Activities {
			ref.topic.Activities = append(ref.topic.Activities, program.Node[program.Activity]{V: a})
		}
	}

	mergeLectures(root, refs, generated, targets)
	mergeAssessment(root, refs, generated, targets)

	if root.Sources == nil {
		root.Sources = []program.Node[program.Source]{}
	}
	if root.NormativeBasis == nil {
		root.NormativeBasis = []string{}
	}
}

// mergeLectures rebuilds the lecture list in topic order, with a new lecture
// for each targeted topic and the existing ones for the rest.
func mergeLectures(root *program.Root, refs []topicRef, generated map[string]TopicContent, targets target) {
	existing := map[string][]program.Node[program.Lecture]{}
	for _, l := range root.Lectures {
		existing[l.V.TopicID] = append(existing[l.V.TopicID], l)
	}

	lectures := []program.Node[program.Lecture]{}
	inCurriculum := map[string]bool{}
	for _, ref := range refs {
		id := ref.topic.ID
		inCurriculum[id] = true

		if targets[id]&partContent == 0 {
			lectures = append(lectures, existing[id]...)
			continue
		}
		if lecture := generated[id].Lecture; lecture != nil {
			l := *lecture
			l.ID = lectureID(id)
			l.TopicID = id
			lectures = append(lectures, program.Node[program.Lecture]{V: l})
		}
	}

	// Lectures of topics that are no longer in the curriculum stay as they were.
	for _, l := range root.Lectures {
		if !inCurriculum[l.V.TopicID] {
			lectures = append(lectures, l)
		}
	}
	root.Lectures = lectures
}

// mergeAssessment rebuilds the tests (one per module, TEST<n>) and the seminar
// tasks (one per topic). Tests this step did not create are left alone.
func mergeAssessment(root *program.Root, refs []topicRef, generated map[string]TopicContent, targets target) {
	if root.Assessment == nil {
		root.Assessment = &program.Node[program.Assessment]{}
	}
	assessment := &root.Assessment.V

	ownedTests := map[string]bool{}
	for _, ref := range refs {
		ownedTests[testID(ref.module.ID)] = true
	}

	// What exists today, by topic, in the tests this step owns.
	oldQuestions := map[string][]program.Node[program.Question]{}
	for _, test := range assessment.Tests {
		if !ownedTests[test.V.ID] {
			continue
		}
		for _, q := range test.V.Questions {
			if len(q.V.TopicIDs) > 0 {
				oldQuestions[q.V.TopicIDs[0]] = append(oldQuestions[q.V.TopicIDs[0]], q)
			}
		}
	}
	oldTasks := map[string][]program.Node[program.SeminarTask]{}
	for _, task := range assessment.SeminarTasks {
		oldTasks[task.V.TopicID] = append(oldTasks[task.V.TopicID], task)
	}

	questionsByModule := map[string][]program.Node[program.Question]{}
	tasks := []program.Node[program.SeminarTask]{}
	inCurriculum := map[string]bool{}

	for _, ref := range refs {
		id := ref.topic.ID
		inCurriculum[id] = true

		if targets[id]&partAssessment == 0 {
			questionsByModule[ref.module.ID] = append(questionsByModule[ref.module.ID], oldQuestions[id]...)
			tasks = append(tasks, oldTasks[id]...)
			continue
		}

		content := generated[id]
		for i, q := range content.Questions {
			q.ID = questionID(id, i)
			q.TopicIDs = []string{id}
			questionsByModule[ref.module.ID] = append(questionsByModule[ref.module.ID], program.Node[program.Question]{V: q})
		}
		if content.Task != nil {
			task := *content.Task
			task.ID = taskID(id)
			task.TopicID = id
			tasks = append(tasks, program.Node[program.SeminarTask]{V: task})
		}
	}
	for _, task := range assessment.SeminarTasks {
		if !inCurriculum[task.V.TopicID] {
			tasks = append(tasks, task)
		}
	}

	// Rebuild the owned tests in place; foreign tests pass through untouched.
	var tests []program.Node[program.Test]
	handled := map[string]bool{}
	for _, test := range assessment.Tests {
		if !ownedTests[test.V.ID] {
			tests = append(tests, test)
			continue
		}
		moduleID := moduleIDOfTest(refs, test.V.ID)
		if questions := questionsByModule[moduleID]; len(questions) > 0 {
			test.V.Questions = questions
			tests = append(tests, test)
		}
		handled[test.V.ID] = true
	}
	for _, ref := range refs {
		id := testID(ref.module.ID)
		if handled[id] || len(questionsByModule[ref.module.ID]) == 0 {
			continue
		}
		handled[id] = true
		moduleID := ref.module.ID
		threshold := 0.6
		tests = append(tests, program.Node[program.Test]{V: program.Test{
			ID:            id,
			Title:         "Текущий контроль: " + ref.module.Title,
			Purpose:       "current",
			ModuleID:      &moduleID,
			PassThreshold: &threshold,
			Questions:     questionsByModule[ref.module.ID],
		}})
	}

	assessment.Tests = tests
	assessment.SeminarTasks = tasks
}

func moduleIDOfTest(refs []topicRef, id string) string {
	for _, ref := range refs {
		if testID(ref.module.ID) == id {
			return ref.module.ID
		}
	}
	return ""
}
