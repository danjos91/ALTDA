package validate

import (
	"math"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

// units is a row of hours in half-hours, as integers: hours move in 0.5
// steps, so comparing integers avoids floating-point surprises.
type units struct {
	lecture, practice, lab, selfStudy, assessment, contact, total int
}

func half(hours float64) int { return int(math.Round(hours * 2)) }

func unitsOf(h program.Hours) units {
	return units{
		lecture:    half(h.Lecture),
		practice:   half(h.Practice),
		lab:        half(h.Lab),
		selfStudy:  half(h.SelfStudy),
		assessment: half(h.Assessment),
		contact:    half(h.Contact),
		total:      half(h.Total),
	}
}

func (u units) plus(o units) units {
	return units{
		lecture:    u.lecture + o.lecture,
		practice:   u.practice + o.practice,
		lab:        u.lab + o.lab,
		selfStudy:  u.selfStudy + o.selfStudy,
		assessment: u.assessment + o.assessment,
		contact:    u.contact + o.contact,
		total:      u.total + o.total,
	}
}

// sameKinds compares the five work kinds (not contact/total).
func (u units) sameKinds(o units) bool {
	return u.lecture == o.lecture && u.practice == o.practice && u.lab == o.lab &&
		u.selfStudy == o.selfStudy && u.assessment == o.assessment
}

// sameAll compares every field, contact and total included.
func (u units) sameAll(o units) bool {
	return u.sameKinds(o) && u.contact == o.contact && u.total == o.total
}

// hoursOf converts half-hours back to hours for messages.
func hoursOf(v int) float64 { return float64(v) / 2 }
