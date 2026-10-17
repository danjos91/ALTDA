package program

// HourKeys are the additive hour components (excluding contact/total).
var HourKeys = []string{"lecture", "practice", "lab", "selfStudy", "assessment"}

// AllHourKeys includes derived contact and total.
var AllHourKeys = []string{"lecture", "practice", "lab", "selfStudy", "assessment", "contact", "total"}

// Half converts academic hours to integer half-hours.
func Half(x float64) int {
	return int(x*2 + 0.5)
}

// FromHalf converts half-hours back to academic hours.
func FromHalf(h int) float64 {
	return float64(h) / 2
}

// Hours holds one hours object.
type Hours struct {
	Lecture    float64 `json:"lecture"`
	Practice   float64 `json:"practice"`
	Lab        float64 `json:"lab"`
	SelfStudy  float64 `json:"selfStudy"`
	Assessment float64 `json:"assessment"`
	Contact    float64 `json:"contact"`
	Total      float64 `json:"total"`
}

// Normalize applies contact = lecture+practice+lab and total = contact+selfStudy+assessment.
func (h *Hours) Normalize() {
	h.Contact = h.Lecture + h.Practice + h.Lab
	h.Total = h.Contact + h.SelfStudy + h.Assessment
}

// ToObject returns a JSON-friendly hours map with normalized derived fields.
func (h Hours) ToObject() Object {
	h.Normalize()
	return Object{
		"lecture":    h.Lecture,
		"practice":   h.Practice,
		"lab":        h.Lab,
		"selfStudy":  h.SelfStudy,
		"assessment": h.Assessment,
		"contact":    h.Contact,
		"total":      h.Total,
	}
}

// HoursFromObject parses an hours object.
func HoursFromObject(o Object) Hours {
	h := Hours{
		Lecture:    GetFloat(o, "lecture", 0),
		Practice:   GetFloat(o, "practice", 0),
		Lab:        GetFloat(o, "lab", 0),
		SelfStudy:  GetFloat(o, "selfStudy", 0),
		Assessment: GetFloat(o, "assessment", 0),
		Contact:    GetFloat(o, "contact", 0),
		Total:      GetFloat(o, "total", 0),
	}
	return h
}

// SumHours sums hour objects component-wise.
func SumHours(rows []Hours) Hours {
	var s Hours
	for _, r := range rows {
		s.Lecture += r.Lecture
		s.Practice += r.Practice
		s.Lab += r.Lab
		s.SelfStudy += r.SelfStudy
		s.Assessment += r.Assessment
		s.Contact += r.Contact
		s.Total += r.Total
	}
	return s
}

// EqualHalf compares selected keys in half-hours.
func EqualHalf(a, b Hours, keys []string) bool {
	ao, bo := a.ToObject(), b.ToObject()
	for _, k := range keys {
		if Half(GetFloat(ao, k, 0)) != Half(GetFloat(bo, k, 0)) {
			return false
		}
	}
	return true
}
