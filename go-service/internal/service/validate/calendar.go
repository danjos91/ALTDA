package validate

import (
	"fmt"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

func validateCalendar(c *checker, root *program.Root) {
	if root.Calendar == nil || len(root.Calendar.V.Sessions) == 0 {
		return
	}
	sessions := root.Calendar.V.Sessions
	const path = "/calendar/sessions"

	numbersOK := true
	for i, s := range sessions {
		if s.V.Number != i+1 {
			numbersOK = false
		}
	}
	numbers := make([]int, len(sessions))
	for i, s := range sessions {
		numbers[i] = s.V.Number
	}
	c.check(ruleCalendarSessionNumbers, numbersOK, path,
		fmt.Sprintf("Session numbers must be 1..%d in order, got %v", len(sessions), numbers))

	topics := topicIDSet(root)
	scheduledTopics := map[string]bool{}
	for _, s := range sessions {
		for _, id := range s.V.TopicIDs {
			if !topics[id] {
				scheduledTopics[id] = true
			}
		}
	}
	bad := sortedKeys(scheduledTopics)
	c.check(ruleDanglingTopic, len(bad) == 0, path,
		fmt.Sprintf("Calendar references non-existent topics: %v", bad), bad...)

	// Self-study is not scheduled in sessions (assumption to confirm with the ADD template).
	scheduled := 0
	for _, s := range sessions {
		scheduled += half(s.V.Hours)
	}
	totals := unitsOf(root.Curriculum.V.Totals)
	expected := totals.total - totals.selfStudy
	c.check(ruleCalendarHours, scheduled == expected, path,
		fmt.Sprintf("Sessions add up to %v h, expected %v h (total minus self-study)",
			hoursOf(scheduled), hoursOf(expected)))
}
