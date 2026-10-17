package allocator

import "math"

// row is a set of hours in half-hour units.
type row struct {
	lecture, practice, lab, selfStudy, assessment int
}

func (r row) contact() int { return r.lecture + r.practice + r.lab }
func (r row) total() int   { return r.contact() + r.selfStudy + r.assessment }

func (r *row) add(o row) {
	r.lecture += o.lecture
	r.practice += o.practice
	r.lab += o.lab
	r.selfStudy += o.selfStudy
	r.assessment += o.assessment
}

func (r row) hours() Hours {
	return Hours{
		Lecture:    toHours(r.lecture),
		Practice:   toHours(r.practice),
		Lab:        toHours(r.lab),
		SelfStudy:  toHours(r.selfStudy),
		Assessment: toHours(r.assessment),
		Contact:    toHours(r.contact()),
		Total:      toHours(r.total()),
	}
}

func rowFromHours(h Hours) row {
	return row{
		lecture:    toUnits(h.Lecture),
		practice:   toUnits(h.Practice),
		lab:        toUnits(h.Lab),
		selfStudy:  toUnits(h.SelfStudy),
		assessment: toUnits(h.Assessment),
	}
}

func toUnits(hours float64) int { return int(math.Round(hours * 2)) }
func toHours(units int) float64 { return float64(units) / 2 }

type kindSet struct{ lecture, practice, lab, selfStudy bool }

func newKindSet(kinds []Kind) kindSet {
	var s kindSet
	for _, k := range kinds {
		switch k {
		case KindLecture:
			s.lecture = true
		case KindPractice:
			s.practice = true
		case KindLab:
			s.lab = true
		case KindSelfStudy:
			s.selfStudy = true
		}
	}
	return s
}

// splitTopic divides a topic's units among the kinds it lists.
// practiceShare is the share of the non-selfStudy hours that is practice/lab;
// selfStudyShare is the share of all the topic's hours that is selfStudy.
func splitTopic(units int, kinds kindSet, practiceShare, selfStudyShare float64) row {
	var r row

	hasPractical := kinds.practice || kinds.lab
	if !kinds.lecture && !hasPractical {
		r.selfStudy = units
		return r
	}

	rest := units
	if kinds.selfStudy && units >= 2 {
		self := int(math.Round(float64(units) * selfStudyShare))
		self = min(max(self, 1), units-1)
		r.selfStudy = self
		rest -= self
	}

	practical := 0
	switch {
	case kinds.lecture && hasPractical:
		parts := distribute(rest, []float64{1 - practiceShare, practiceShare})
		r.lecture, practical = parts[0], parts[1]
		// A listed kind must not vanish when there is room for both.
		if rest >= 2 && r.lecture == 0 {
			r.lecture, practical = 1, practical-1
		}
		if rest >= 2 && practical == 0 {
			r.lecture, practical = r.lecture-1, 1
		}
	case kinds.lecture:
		r.lecture = rest
	default:
		practical = rest
	}

	switch {
	case kinds.practice && kinds.lab:
		r.lab = practical / 2
		r.practice = practical - r.lab
	case kinds.lab:
		r.lab = practical
	default:
		r.practice = practical
	}
	return r
}
