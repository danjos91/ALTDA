package validate

import (
	"slices"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

// minAssessmentsPerOutcome: tests + seminar tasks + final attestation.
const minAssessmentsPerOutcome = 2

const minutesPerAcademicHour = 45

// topicsOf lists every topic of the curriculum in order.
func topicsOf(root *program.Root) []*program.Topic {
	var topics []*program.Topic
	if root.Curriculum == nil {
		return nil
	}
	modules := root.Curriculum.V.Modules
	for mi := range modules {
		for ti := range modules[mi].V.Topics {
			topics = append(topics, &modules[mi].V.Topics[ti].V)
		}
	}
	return topics
}

func topicIDSet(root *program.Root) map[string]bool {
	ids := map[string]bool{}
	for _, t := range topicsOf(root) {
		ids[t.ID] = true
	}
	return ids
}

// outcomeSet is every outcome the competencies define.
func outcomeSet(root *program.Root) map[string]bool {
	ids := map[string]bool{}
	for _, c := range root.Competencies {
		for _, id := range c.V.OutcomeIDs() {
			ids[id] = true
		}
	}
	return ids
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func statusAtLeast(root *program.Root, status string) bool {
	return slices.Index(statusOrder, root.Status) >= slices.Index(statusOrder, status)
}

func hasAssessment(root *program.Root) bool {
	if root.Assessment == nil {
		return false
	}
	a := root.Assessment.V
	return len(a.Tests) > 0 || len(a.SeminarTasks) > 0
}
