package content

import (
	"math"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

func halves(hours float64) int { return int(math.Round(hours * 2)) }

// scheduleItem is a block of hours to place in sessions.
type scheduleItem struct {
	topicID     string // empty for hours that belong to no topic
	units       int    // half-hours
	attestation bool
}

type session struct {
	units       int
	topics      []string
	attestation bool
}

// buildCalendar places the program's contact and assessment hours, in order,
// into sessions of at most sessionHours. Self-study is not scheduled. The
// sessions therefore add up to total minus self-study, which is what the
// validator expects.
func buildCalendar(root *program.Root, sessionHours, sessionsPerWeek float64) []program.Node[program.Session] {
	var items []scheduleItem

	modules := root.Curriculum.V.Modules
	for mi := range modules {
		module := &modules[mi].V
		topicAssessment := 0
		for ti := range module.Topics {
			h := module.Topics[ti].V.Hours
			topicAssessment += halves(h.Assessment)
			if units := halves(h.Lecture) + halves(h.Practice) + halves(h.Lab) + halves(h.Assessment); units > 0 {
				items = append(items, scheduleItem{topicID: module.Topics[ti].V.ID, units: units})
			}
		}
		// Assessment held only by the module row.
		if extra := halves(module.Hours.Assessment) - topicAssessment; extra > 0 {
			items = append(items, scheduleItem{units: extra})
		}
	}
	if units := halves(root.FinalAttestation.V.Hours.Assessment); units > 0 {
		items = append(items, scheduleItem{units: units, attestation: true})
	}

	capacity := max(halves(sessionHours), 1)
	var sessions []*session
	var current *session
	remaining := 0

	for _, item := range items {
		left := item.units
		for left > 0 {
			if current == nil || remaining == 0 {
				current = &session{}
				sessions = append(sessions, current)
				remaining = capacity
			}
			take := min(left, remaining)
			current.units += take
			if item.topicID != "" && !contains(current.topics, item.topicID) {
				current.topics = append(current.topics, item.topicID)
			}
			if item.attestation {
				current.attestation = true
			}
			left -= take
			remaining -= take
		}
	}

	if sessionsPerWeek <= 0 {
		sessionsPerWeek = 1
	}
	nodes := make([]program.Node[program.Session], len(sessions))
	for i, s := range sessions {
		week := int(math.Ceil(float64(i+1) / sessionsPerWeek))
		topics := s.topics
		if topics == nil {
			topics = []string{}
		}
		session := program.Session{Number: i + 1, Week: &week, Hours: float64(s.units) / 2, TopicIDs: topics}
		if s.attestation {
			yes := true
			session.FinalAttestation = &yes
		}
		nodes[i] = program.Node[program.Session]{V: session}
	}
	return nodes
}

// sessionsByTopic lists, for each topic, the sessions that include it.
func sessionsByTopic(sessions []program.Node[program.Session]) map[string][]int {
	out := map[string][]int{}
	for _, s := range sessions {
		for _, id := range s.V.TopicIDs {
			out[id] = append(out[id], s.V.Number)
		}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
