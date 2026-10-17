package validate

import (
	"fmt"
	"math"
	"slices"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

// weightTolerance is how far a sum of weights may be from 1.
const weightTolerance = 0.01

// CheckDraft runs the skeleton-step rules on one draft, before hours are
// allocated. It is the code-side check of what a drafter (template or LLM)
// proposes. A failed check with error severity means the draft is unusable.
func CheckDraft(d skeleton.Draft) []Check {
	c := &checker{checks: []Check{}}
	label := "variant " + d.Key

	var moduleWeights float64
	for _, m := range d.Modules {
		moduleWeights += m.Weight
	}
	c.check(ruleSkeletonModuleWeights, math.Abs(moduleWeights-1) <= weightTolerance, "",
		fmt.Sprintf("%s: module weights add up to %.3f, expected 1", label, moduleWeights), d.Key)

	for _, m := range d.Modules {
		var topicWeights float64
		for _, t := range m.Topics {
			topicWeights += t.Weight
		}
		c.check(ruleSkeletonTopicWeights, math.Abs(topicWeights-1) <= weightTolerance, "",
			fmt.Sprintf("%s/%s: topic weights add up to %.3f, expected 1", label, m.ID, topicWeights), m.ID)
	}

	defined := d.OutcomeIDs()
	var used []string
	position := map[string]int{}
	for _, m := range d.Modules {
		for _, t := range m.Topics {
			position[t.ID] = len(position)
			used = append(used, t.OutcomeIDs...)
		}
	}

	unknown := difference(used, defined)
	c.check(ruleSkeletonDanglingOutcome, len(unknown) == 0, "",
		fmt.Sprintf("%s: unknown outcomes %v", label, unknown), unknown...)

	untaught := difference(defined, used)
	c.check(ruleCoverageOutcomeTaught, len(untaught) == 0, "",
		fmt.Sprintf("%s: outcomes not in any topic %v", label, untaught), untaught...)

	var late []string
	for _, m := range d.Modules {
		for _, t := range m.Topics {
			for _, prerequisite := range t.Prerequisites {
				at, known := position[prerequisite]
				if !known || at >= position[t.ID] {
					late = append(late, t.ID+" <- "+prerequisite)
				}
			}
		}
	}
	c.check(ruleOrderPrerequisites, len(late) == 0, "",
		fmt.Sprintf("%s: bad prerequisites %v", label, late), late...)

	return c.checks
}

// difference lists the values of a that are not in b, sorted and unique.
func difference(a, b []string) []string {
	inB := make(map[string]bool, len(b))
	for _, v := range b {
		inB[v] = true
	}

	var out []string
	for _, v := range a {
		if !inB[v] && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return out
}

// Failed returns the error-severity checks that did not pass.
func Failed(checks []Check) []Check {
	var failed []Check
	for _, c := range checks {
		if !c.Passed && c.Severity == SeverityError {
			failed = append(failed, c)
		}
	}
	return failed
}
