package validate

import (
	"fmt"
	"math"
	"slices"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

func validateContent(c *checker, root *program.Root) {
	topics := map[string]*program.Topic{}
	for _, t := range topicsOf(root) {
		topics[t.ID] = t
	}

	// A topic's lecture plans add up to its lecture hours x 45 minutes. Python's
	// round() goes to the even neighbour on .5 (22.5 -> 22), and so does this.
	minutes := map[string]int{}
	var order []string
	for _, l := range root.Lectures {
		if _, seen := minutes[l.V.TopicID]; !seen {
			order = append(order, l.V.TopicID)
		}
		for _, step := range l.V.Plan {
			minutes[l.V.TopicID] += step.V.Minutes
		}
		if _, seen := minutes[l.V.TopicID]; !seen {
			minutes[l.V.TopicID] = 0
		}
	}
	for _, id := range order {
		topic, known := topics[id]
		if !known {
			continue
		}
		expected := int(math.RoundToEven(topic.Hours.Lecture * minutesPerAcademicHour))
		c.check(ruleLectureMinutes, minutes[id] == expected, "",
			fmt.Sprintf("%s: lecture plans add up to %d min, expected %d min", id, minutes[id], expected), id)
	}

	if root.Assessment == nil {
		return
	}

	for _, test := range root.Assessment.V.Tests {
		for _, q := range test.V.Questions {
			ok, message := testKeyOK(q.V)
			c.check(ruleTestKey, ok, "", fmt.Sprintf("%s/%s: %s", test.V.ID, q.V.ID, message), q.V.ID)
		}
	}

	// Seminar tasks: the score adds up, and the tasks of a topic fit its practice + lab hours.
	perTopic := map[string]int{}
	var topicOrder []string
	for _, task := range root.Assessment.V.SeminarTasks {
		total := 0
		for _, criterion := range task.V.Criteria {
			total += criterion.V.MaxPoints
		}
		c.check(ruleTaskMaxScore, total == task.V.MaxScore, "",
			fmt.Sprintf("%s: maxScore=%d, criteria add up to %d", task.V.ID, task.V.MaxScore, total), task.V.ID)

		if _, seen := perTopic[task.V.TopicID]; !seen {
			topicOrder = append(topicOrder, task.V.TopicID)
		}
		perTopic[task.V.TopicID] += half(task.V.DurationHours)
	}
	for _, id := range topicOrder {
		topic, known := topics[id]
		if !known {
			continue
		}
		capacity := half(topic.Hours.Practice) + half(topic.Hours.Lab)
		c.check(ruleTaskDuration, perTopic[id] <= capacity, "",
			fmt.Sprintf("%s: tasks take %v h, topic has %v h of practice + lab",
				id, hoursOf(perTopic[id]), hoursOf(capacity)), id)
	}
}

// testKeyOK checks the answer key of one question.
func testKeyOK(q program.Question) (bool, string) {
	if q.Type == "short" {
		return true, "ok"
	}

	optionIDs := make([]string, len(q.Options))
	unique := map[string]bool{}
	for i, o := range q.Options {
		optionIDs[i] = o.V.ID
		unique[o.V.ID] = true
	}
	if len(unique) != len(optionIDs) {
		return false, "duplicate option ids"
	}

	var unknown []string
	for _, answer := range q.Correct {
		if !slices.Contains(optionIDs, answer) {
			unknown = append(unknown, answer)
		}
	}
	if len(unknown) > 0 {
		return false, fmt.Sprintf("correct answers not among the options: %v", unknown)
	}

	switch q.Type {
	case "single", "truefalse":
		return len(q.Correct) == 1, fmt.Sprintf("%d correct answers (must be exactly 1)", len(q.Correct))
	case "multiple":
		return len(q.Correct) >= 2 && len(q.Correct) < len(optionIDs),
			fmt.Sprintf("%d correct of %d options (need 2+, and not all)", len(q.Correct), len(optionIDs))
	}
	return true, "ok"
}
