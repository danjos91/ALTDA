// Package allocator turns the LLM's relative weights into exact hours.
// It is deterministic and never calls a model: the LLM proposes shares, this
// code makes the sums come out right.
package allocator

import (
	"fmt"
	"math"
)

type topicSlot struct {
	kinds kindSet
	row   row
	fixed bool
}

// Allocate distributes in.TotalHours over the modules, topics and kinds.
// The result satisfies, at every level, contact = lecture+practice+lab and
// total = contact+selfStudy+assessment, and the grand total equals
// in.TotalHours exactly. Fixed topics keep their hours; the rest share what
// is left.
func Allocate(in Input, cfg Config) (*Result, error) {
	if err := validate(in); err != nil {
		return nil, err
	}

	total := toUnits(in.TotalHours)
	minTopic := max(toUnits(cfg.MinTopicHours), 1)

	freeTopics, fixedUnits := 0, 0
	for _, m := range in.Modules {
		for _, t := range m.Topics {
			if t.Fixed != nil {
				fixedUnits += rowFromHours(*t.Fixed).total()
			} else {
				freeTopics++
			}
		}
	}
	needed := fixedUnits + freeTopics*minTopic

	var warnings []string
	attestationFixed := in.FixedAttestation != nil
	var attestation int
	if attestationFixed {
		attestation = toUnits(*in.FixedAttestation)
	} else {
		attestation = attestationUnits(total, cfg)
	}

	room := total - needed // what the attestation and the extra hours can use
	switch {
	case attestationFixed && room < attestation, !attestationFixed && room < 1:
		return nil, fmt.Errorf("%w: %d free topics and %v h locked need at least %v h, program has %v h",
			ErrNotEnoughHours, freeTopics, toHours(fixedUnits), toHours(needed+max(attestation, 1)), in.TotalHours)
	case !attestationFixed && attestation > room:
		attestation = room
		warnings = append(warnings, "Часы итоговой аттестации сокращены, чтобы хватило часов на все темы.")
	}
	body := total - attestation

	if freeTopics == 0 && body != fixedUnits {
		return nil, fmt.Errorf("%w: locked topics have %v h plus %v h of attestation, the program needs %v h",
			ErrNothingToBalance, toHours(fixedUnits), toHours(attestation), in.TotalHours)
	}

	moduleUnits := splitModules(in.Modules, body, minTopic)

	slots := make([][]*topicSlot, len(in.Modules))
	for mi, m := range in.Modules {
		topicUnits := splitTopics(m.Topics, moduleUnits[mi], minTopic)
		slots[mi] = make([]*topicSlot, len(m.Topics))
		for ti, t := range m.Topics {
			slots[mi][ti] = newSlot(in, cfg, m, t, topicUnits[ti])
		}
	}

	if in.MinPracticeShare != nil && !in.IsInternship {
		if !raisePractice(slots, total, *in.MinPracticeShare) {
			warnings = append(warnings, fmt.Sprintf(
				"Не удалось достичь доли практики %.0f%% при заданном составе тем.", *in.MinPracticeShare*100))
		}
	}

	results := make([]ModuleResult, len(in.Modules))
	var grand row
	for mi, m := range in.Modules {
		var moduleRow row
		topics := make([]TopicResult, len(m.Topics))
		for ti, t := range m.Topics {
			moduleRow.add(slots[mi][ti].row)
			topics[ti] = TopicResult{ID: t.ID, Hours: slots[mi][ti].row.hours()}
		}
		results[mi] = ModuleResult{ID: m.ID, Hours: moduleRow.hours(), Topics: topics}
		grand.add(moduleRow)
	}

	attestationRow := row{assessment: attestation}
	grand.add(attestationRow)

	return &Result{
		Modules:          results,
		FinalAttestation: attestationRow.hours(),
		Totals:           grand.hours(),
		Warnings:         warnings,
	}, nil
}

func validate(in Input) error {
	if !(in.TotalHours > 0) || math.IsInf(in.TotalHours, 0) {
		return fmt.Errorf("%w: totalHours must be positive", ErrInvalidInput)
	}
	if len(in.Modules) == 0 {
		return fmt.Errorf("%w: no modules", ErrInvalidInput)
	}
	for _, m := range in.Modules {
		if len(m.Topics) == 0 {
			return fmt.Errorf("%w: module %s has no topics", ErrInvalidInput, m.ID)
		}
		if hasFreeTopic(m) && !(m.Weight > 0) {
			return fmt.Errorf("%w: module %s needs a positive weight", ErrInvalidInput, m.ID)
		}
		for _, t := range m.Topics {
			if t.Fixed != nil {
				continue
			}
			if !(t.Weight > 0) {
				return fmt.Errorf("%w: topic %s needs a positive weight", ErrInvalidInput, t.ID)
			}
			if len(t.Kinds) == 0 {
				return fmt.Errorf("%w: topic %s has no kinds", ErrInvalidInput, t.ID)
			}
		}
	}
	return nil
}

func hasFreeTopic(m Module) bool {
	for _, t := range m.Topics {
		if t.Fixed == nil {
			return true
		}
	}
	return false
}

func attestationUnits(total int, cfg Config) int {
	hours := float64(total) / 2 * cfg.FinalAttestationShare
	hours = math.Min(math.Max(hours, cfg.FinalAttestationMin), cfg.FinalAttestationMax)
	return max(toUnits(hours), 1)
}

// moduleFloor is the least a module can get: its fixed topics as they are,
// plus the minimum for each free one.
func moduleFloor(m Module, minTopic int) int {
	floor := 0
	for _, t := range m.Topics {
		if t.Fixed != nil {
			floor += rowFromHours(*t.Fixed).total()
		} else {
			floor += minTopic
		}
	}
	return floor
}

// splitModules gives every module its floor, then shares what is left by
// weight among the modules that still have free topics.
func splitModules(modules []Module, body, minTopic int) []int {
	floors := make([]int, len(modules))
	weights := make([]float64, len(modules))
	floorSum := 0
	for i, m := range modules {
		floors[i] = moduleFloor(m, minTopic)
		floorSum += floors[i]
		if hasFreeTopic(m) {
			weights[i] = m.Weight
		}
	}

	extra := distribute(body-floorSum, weights)
	out := make([]int, len(modules))
	for i := range modules {
		out[i] = floors[i] + extra[i]
	}
	return out
}

// splitTopics returns each topic's units: fixed ones as they are, free ones
// the minimum plus a weighted share of the module's remainder.
func splitTopics(topics []Topic, moduleUnits, minTopic int) []int {
	out := make([]int, len(topics))
	var freeIdx []int
	var weights []float64
	remainder := moduleUnits

	for i, t := range topics {
		if t.Fixed != nil {
			out[i] = rowFromHours(*t.Fixed).total()
			remainder -= out[i]
			continue
		}
		freeIdx = append(freeIdx, i)
		weights = append(weights, t.Weight)
		remainder -= minTopic
	}

	extra := distribute(remainder, weights)
	for k, i := range freeIdx {
		out[i] = minTopic + extra[k]
	}
	return out
}

func newSlot(in Input, cfg Config, m Module, t Topic, units int) *topicSlot {
	if t.Fixed != nil {
		return &topicSlot{kinds: newKindSet(t.Kinds), row: rowFromHours(*t.Fixed), fixed: true}
	}
	return &topicSlot{kinds: newKindSet(t.Kinds), row: splitOne(in, cfg, m, t, units)}
}

func splitOne(in Input, cfg Config, m Module, t Topic, units int) row {
	if in.IsInternship {
		return row{selfStudy: units}
	}

	practiceShare := cfg.DefaultPracticeShare
	if m.IsIntro {
		practiceShare = cfg.IntroPracticeShare
	}
	if t.PracticeShareHint != nil {
		practiceShare = *t.PracticeShareHint
	}
	practiceShare = math.Min(math.Max(practiceShare, 0), 1)

	selfShare, ok := cfg.SelfStudyShare[in.StudyForm]
	if !ok {
		selfShare = cfg.SelfStudyShare["очная"]
	}
	return splitTopic(units, newKindSet(t.Kinds), practiceShare, selfShare)
}

// raisePractice moves half-hours from lecture to practice/lab, one at a time
// from the free topic with the most lecture, until (practice+lab)/total
// reaches minShare. It reports whether it got there.
func raisePractice(slots [][]*topicSlot, total int, minShare float64) bool {
	practical := func() int {
		sum := 0
		for _, ms := range slots {
			for _, s := range ms {
				sum += s.row.practice + s.row.lab
			}
		}
		return sum
	}

	// Compare in integers: practical/total >= minShare <=> practical >= ceil(minShare*total).
	need := int(math.Ceil(minShare*float64(total) - 1e-9))

	for have := practical(); have < need; have++ {
		var best *topicSlot
		for _, ms := range slots {
			for _, s := range ms {
				canMove := !s.fixed && s.row.lecture > 1 && (s.kinds.practice || s.kinds.lab)
				if canMove && (best == nil || s.row.lecture > best.row.lecture) {
					best = s
				}
			}
		}
		if best == nil {
			return false
		}

		best.row.lecture--
		if best.kinds.practice {
			best.row.practice++
		} else {
			best.row.lab++
		}
	}
	return true
}
