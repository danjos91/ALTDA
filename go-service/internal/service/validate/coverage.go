package validate

import (
	"fmt"
	"strings"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

func validateCoverage(c *checker, root *program.Root) {
	outcomes := outcomeSet(root)

	taught := map[string]bool{}
	for _, t := range topicsOf(root) {
		for _, id := range t.OutcomeIDs {
			taught[id] = true
		}
	}

	// Assessment tools per outcome: every question, seminar task and the final
	// attestation counts once for each outcome it names.
	var uses [][]string
	if root.Assessment != nil {
		for _, test := range root.Assessment.V.Tests {
			for _, q := range test.V.Questions {
				uses = append(uses, q.V.OutcomeIDs)
			}
		}
		for _, task := range root.Assessment.V.SeminarTasks {
			uses = append(uses, task.V.OutcomeIDs)
		}
	}
	if root.FinalAttestation != nil {
		uses = append(uses, root.FinalAttestation.V.OutcomeIDs)
	}

	tools := map[string]int{}
	for _, ids := range uses {
		seen := map[string]bool{}
		for _, id := range ids {
			if outcomes[id] && !seen[id] {
				seen[id] = true
				tools[id]++
			}
		}
	}

	assessedStage := statusAtLeast(root, "content_ready") || hasAssessment(root)
	for _, id := range sortedKeys(outcomes) {
		c.check(ruleCoverageOutcomeTaught, taught[id], "",
			fmt.Sprintf("%s does not appear in any topic", id), id)
		if assessedStage {
			c.check(ruleCoverageOutcomeAssessed, tools[id] >= minAssessmentsPerOutcome, "",
				fmt.Sprintf("%s has %d assessment tool(s), needs >= %d", id, tools[id], minAssessmentsPerOutcome), id)
		}
	}

	// Outcome IDs must start with their competency's ID.
	for _, comp := range root.Competencies {
		var bad []string
		for _, id := range comp.V.OutcomeIDs() {
			if strings.SplitN(id, ".", 2)[0] != comp.V.ID {
				bad = append(bad, id)
			}
		}
		c.check(ruleOutcomePrefix, len(bad) == 0, "",
			fmt.Sprintf("%s: outcomes with a foreign prefix: %v", comp.V.ID, bad), bad...)
	}

	referenced := map[string]bool{}
	for id := range taught {
		referenced[id] = true
	}
	for _, ids := range uses {
		for _, id := range ids {
			referenced[id] = true
		}
	}
	for _, lecture := range root.Lectures {
		for _, id := range lecture.V.OutcomeIDs {
			referenced[id] = true
		}
	}
	dangling := difference(sortedKeys(referenced), sortedKeys(outcomes))
	c.check(ruleDanglingOutcome, len(dangling) == 0, "",
		fmt.Sprintf("Non-existent outcomes referenced: %v", dangling), dangling...)
}
