package validate

import (
	"fmt"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

func validateReferences(c *checker, root *program.Root) {
	topics := topicIDSet(root)
	list := topicsOf(root)

	topicRefs := map[string]bool{}
	for _, l := range root.Lectures {
		topicRefs[l.V.TopicID] = true
	}
	if root.Assessment != nil {
		for _, task := range root.Assessment.V.SeminarTasks {
			topicRefs[task.V.TopicID] = true
		}
		for _, test := range root.Assessment.V.Tests {
			for _, q := range test.V.Questions {
				for _, id := range q.V.TopicIDs {
					topicRefs[id] = true
				}
			}
		}
	}
	for _, s := range root.Sources {
		for _, id := range s.V.TopicIDs {
			topicRefs[id] = true
		}
	}
	for _, tool := range root.Tools {
		for _, id := range tool.V.UsedInTopics {
			topicRefs[id] = true
		}
	}
	for _, t := range list {
		for _, id := range t.Prerequisites {
			topicRefs[id] = true
		}
	}
	badTopics := difference(sortedKeys(topicRefs), sortedKeys(topics))
	c.check(ruleDanglingTopic, len(badTopics) == 0, "",
		fmt.Sprintf("Non-existent topics referenced: %v", badTopics), badTopics...)

	// Prerequisites must come earlier in the curriculum (this also rules out cycles).
	position := map[string]int{}
	for i, t := range list {
		position[t.ID] = i
	}
	var late []string
	for _, t := range list {
		for _, p := range t.Prerequisites {
			if at, known := position[p]; known && at >= position[t.ID] {
				late = append(late, t.ID+" <- "+p)
			}
		}
	}
	c.check(ruleOrderPrerequisites, len(late) == 0, "",
		fmt.Sprintf("Prerequisites that do not come earlier: %v", late), late...)

	// References from the methodologist cited but missing from request.attachments.
	attachments := map[string]bool{}
	if root.Request != nil {
		for _, a := range root.Request.V.Attachments {
			attachments[a.V.ID] = true
		}
	}
	usedRefs := map[string]bool{}
	for _, t := range list {
		for _, id := range t.ReferenceIDs {
			usedRefs[id] = true
		}
	}
	for _, l := range root.Lectures {
		for _, id := range l.V.ReferenceIDs {
			usedRefs[id] = true
		}
	}
	badRefs := difference(sortedKeys(usedRefs), sortedKeys(attachments))
	c.check(ruleDanglingReference, len(badRefs) == 0, "",
		fmt.Sprintf("Non-existent references cited: %v", badRefs), badRefs...)

	// Sources cited but missing from sources[].
	sourceIDs := map[string]bool{}
	for _, s := range root.Sources {
		sourceIDs[s.V.ID] = true
	}
	usedSources := map[string]bool{}
	for _, id := range root.NormativeBasis {
		usedSources[id] = true
	}
	for _, l := range root.Lectures {
		for _, id := range l.V.SourceIDs {
			usedSources[id] = true
		}
	}
	badSources := difference(sortedKeys(usedSources), sortedKeys(sourceIDs))
	c.check(ruleDanglingSource, len(badSources) == 0, "",
		fmt.Sprintf("Non-existent sources cited: %v", badSources), badSources...)

	validateProfStandardLinks(c, root)
}

// validateProfStandardLinks: a competency's link must point to a profstandard
// (and labor function) listed in dpp. Only ДПП ПК programs have them.
func validateProfStandardLinks(c *checker, root *program.Root) {
	if root.ProgramType != "DPP_PK" {
		return
	}

	standards := map[string]map[string]bool{}
	if root.Dpp != nil {
		for _, ps := range root.Dpp.V.ProfStandards {
			functions := map[string]bool{}
			for _, f := range ps.V.LaborFunctions {
				functions[f.V.Code] = true
			}
			standards[ps.V.RegistryRef] = functions
		}
	}

	var bad []string
	for _, comp := range root.Competencies {
		for _, link := range comp.V.ProfStandardLinks {
			functions, known := standards[link.V.RegistryRef]
			if !known || (len(functions) > 0 && !functions[link.V.LaborFunctionCode]) {
				bad = append(bad, fmt.Sprintf("%s: %s/%s", comp.V.ID, link.V.RegistryRef, link.V.LaborFunctionCode))
			}
		}
	}
	c.check(ruleProfStandardLink, len(bad) == 0, "",
		fmt.Sprintf("Competency links not found in dpp.profStandards: %v", bad), bad...)
}
