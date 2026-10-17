package content

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

// part says what to regenerate for a topic.
type part uint8

const (
	partContent    part = 1 << iota // activities and the lecture
	partAssessment                  // questions and the seminar task
	partAll        = partContent | partAssessment
)

// target maps a topic ID to what is to be regenerated for it.
type target map[string]part

// topicRef is a topic with the module it sits in.
type topicRef struct {
	module *program.Module
	topic  *program.Topic
}

var (
	moduleIDRe  = regexp.MustCompile(`^M\d+$`)
	topicIDRe   = regexp.MustCompile(`^T\d+\.\d+$`)
	outcomeIDRe = regexp.MustCompile(`^C\d+\.[KUN]\d+$`)
	lectureIDRe = regexp.MustCompile(`^L\d+(\.\d+)*$`)
	taskIDRe    = regexp.MustCompile(`^ST\d+$`)
	testIDRe    = regexp.MustCompile(`^TEST\d+$`)
)

// topicRefs lists every topic of the curriculum in order.
func topicRefs(root *program.Root) []topicRef {
	var refs []topicRef
	modules := root.Curriculum.V.Modules
	for mi := range modules {
		module := &modules[mi].V
		for ti := range module.Topics {
			refs = append(refs, topicRef{module: module, topic: &module.Topics[ti].V})
		}
	}
	return refs
}

// resolveTargets turns node IDs into topics. An ID names a module, a topic, an
// outcome, a lecture, a seminar task, a test, a question, or "assessment" (all
// questions and tasks). Nothing given means the whole program.
func resolveTargets(root *program.Root, nodeIDs []string) (target, error) {
	refs := topicRefs(root)

	result := target{}
	if len(nodeIDs) == 0 {
		for _, r := range refs {
			result[r.topic.ID] = partAll
		}
		return result, nil
	}

	add := func(topicID string, p part) { result[topicID] |= p }

	for _, id := range nodeIDs {
		found := false

		switch {
		case id == "assessment":
			for _, r := range refs {
				add(r.topic.ID, partAssessment)
			}
			found = true

		case moduleIDRe.MatchString(id):
			for _, r := range refs {
				if r.module.ID == id {
					add(r.topic.ID, partAll)
					found = true
				}
			}

		case topicIDRe.MatchString(id):
			for _, r := range refs {
				if r.topic.ID == id {
					add(id, partAll)
					found = true
				}
			}

		case outcomeIDRe.MatchString(id):
			if outcomeExists(root, id) {
				found = true
				for _, r := range refs {
					for _, oid := range r.topic.OutcomeIDs {
						if oid == id {
							add(r.topic.ID, partAll)
						}
					}
				}
			}

		case lectureIDRe.MatchString(id):
			for _, l := range root.Lectures {
				if l.V.ID == id {
					add(l.V.TopicID, partContent)
					found = true
				}
			}

		case taskIDRe.MatchString(id):
			if root.Assessment != nil {
				for _, task := range root.Assessment.V.SeminarTasks {
					if task.V.ID == id {
						add(task.V.TopicID, partAssessment)
						found = true
					}
				}
			}

		case testIDRe.MatchString(id):
			if root.Assessment != nil {
				for _, test := range root.Assessment.V.Tests {
					if test.V.ID != id {
						continue
					}
					found = true
					for _, r := range refs {
						if test.V.ModuleID == nil || *test.V.ModuleID == r.module.ID {
							add(r.topic.ID, partAssessment)
						}
					}
				}
			}

		case strings.HasPrefix(id, "Q"):
			if root.Assessment != nil {
				for _, test := range root.Assessment.V.Tests {
					for _, q := range test.V.Questions {
						if q.V.ID == id {
							for _, topicID := range q.V.TopicIDs {
								add(topicID, partAssessment)
							}
							found = true
						}
					}
				}
			}
		}

		if !found {
			return nil, fmt.Errorf("%w: %q", ErrUnknownNode, id)
		}
	}
	return result, nil
}

func outcomeExists(root *program.Root, id string) bool {
	for _, c := range root.Competencies {
		for _, oid := range c.V.OutcomeIDs() {
			if oid == id {
				return true
			}
		}
	}
	return false
}
