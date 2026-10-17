package allocator

import (
	"math"
	"sort"

	"github.com/danjos91/ALTDA/go-service/internal/program"
)

// Options control allocation behaviour.
type Options struct {
	// LockedIDs: topic/module ids whose hours must not change.
	LockedIDs map[string]bool
	// FinalAttestationHours overrides the default FA reservation (0 = auto).
	FinalAttestationHours float64
}

type topicRef struct {
	modIdx   int
	topicIdx int
	weight   float64
	locked   bool
}

// AllocateProgram rebalances hours on program.curriculum (+ finalAttestation) so
// totals equal header/requested totalHours. Locked rows keep their existing hours.
func AllocateProgram(prog program.Object, opts Options) {
	if prog == nil {
		return
	}
	total := program.TotalHoursFromProgram(prog)
	if total <= 0 {
		return
	}
	if opts.LockedIDs == nil {
		opts.LockedIDs = map[string]bool{}
	}

	curriculum := program.EnsureObject(prog, "curriculum")
	modules := ensureModules(curriculum)
	fa := program.EnsureObject(prog, "finalAttestation")

	faHours := opts.FinalAttestationHours
	if faHours <= 0 {
		if opts.LockedIDs["finalAttestation"] {
			faHours = program.HoursFromObject(program.EnsureObject(fa, "hours")).Total
		} else {
			faHours = reserveAttestation(total)
		}
	}
	faHours = roundHalf(faHours)
	if faHours > total-0.5 {
		faHours = roundHalf(math.Max(0.5, total-0.5))
	}

	refs := collectTopicRefs(modules, opts.LockedIDs)

	lockedHalf := 0
	for _, ref := range refs {
		if !ref.locked {
			continue
		}
		to := topicAt(modules, ref)
		h := program.HoursFromObject(program.EnsureObject(to, "hours"))
		lockedHalf += program.Half(h.Total)
	}
	if opts.LockedIDs["finalAttestation"] {
		// FA already reserved outside the module pool.
	}

	allocatableHalf := program.Half(total) - program.Half(faHours)
	if allocatableHalf < 0 {
		allocatableHalf = 0
	}
	freeHalf := allocatableHalf - lockedHalf
	if freeHalf < 0 {
		freeHalf = 0
	}

	var weights []float64
	var freeIdx []int
	for i, ref := range refs {
		if ref.locked {
			continue
		}
		freeIdx = append(freeIdx, i)
		weights = append(weights, ref.weight)
	}
	shares := largestRemainder(weights, freeHalf)

	sharePos := 0
	for i, ref := range refs {
		to := topicAt(modules, ref)
		if ref.locked {
			h := program.HoursFromObject(program.EnsureObject(to, "hours"))
			h.Normalize()
			to["hours"] = h.ToObject()
			continue
		}
		half := 0
		if sharePos < len(shares) {
			half = shares[sharePos]
			sharePos++
		}
		_ = i
		th := splitTopicHours(to, program.FromHalf(half))
		to["hours"] = th.ToObject()
	}

	for _, mod := range modules {
		rollupModuleHours(mod, opts.LockedIDs)
	}

	if !opts.LockedIDs["finalAttestation"] {
		fah := program.Hours{Assessment: faHours}
		fah.Normalize()
		fa["hours"] = fah.ToObject()
	} else {
		existing := program.HoursFromObject(program.EnsureObject(fa, "hours"))
		existing.Normalize()
		fa["hours"] = existing.ToObject()
	}

	recomputeTotals(curriculum, modules, fa)
}

// AllocateVariant fills hours/totals on a skeleton variant (modules + finalAttestation).
func AllocateVariant(variant program.Object, totalHours float64) {
	if variant == nil {
		return
	}
	shell := program.Object{
		"header": program.Object{"totalHours": totalHours},
	}
	curriculum := program.Object{}
	if modulesAny, ok := program.AsArray(variant["modules"]); ok {
		// Deep-clone so we don't share maps unexpectedly.
		cloned := program.Clone(modulesAny)
		curriculum["modules"] = cloned
	}
	shell["curriculum"] = curriculum
	if fa, ok := program.AsObject(variant["finalAttestation"]); ok {
		shell["finalAttestation"] = program.CloneObject(fa)
	} else {
		shell["finalAttestation"] = program.Object{
			"form":        "зачёт",
			"description": "",
		}
	}
	AllocateProgram(shell, Options{})
	if cur, ok := program.AsObject(shell["curriculum"]); ok {
		variant["modules"] = cur["modules"]
		variant["totals"] = cur["totals"]
	}
	if fa, ok := program.AsObject(shell["finalAttestation"]); ok {
		variant["finalAttestation"] = fa
	}
}

func ensureModules(curriculum program.Object) []program.Object {
	modulesAny := program.EnsureArray(curriculum, "modules")
	modules := make([]program.Object, 0, len(modulesAny))
	for _, m := range modulesAny {
		if mo, ok := program.AsObject(m); ok {
			modules = append(modules, mo)
		}
	}
	if len(modules) == 0 {
		mod := program.Object{
			"id":             "M1",
			"title":          "Модуль 1",
			"isIntro":        false,
			"weight":         1.0,
			"assessmentForm": "нет",
			"topics": program.Array{
				program.Object{
					"id":         "T1.1",
					"title":      "Тема 1.1",
					"weight":     1.0,
					"kinds":      program.Array{"lecture", "practice"},
					"outcomeIds": program.Array{},
				},
			},
		}
		modules = []program.Object{mod}
		curriculum["modules"] = program.Array{mod}
		return modules
	}
	// Write objects back so mutations stick if originals were map[string]any.
	out := make(program.Array, len(modules))
	for i, m := range modules {
		out[i] = m
	}
	curriculum["modules"] = out
	return modules
}

func collectTopicRefs(modules []program.Object, lockedIDs map[string]bool) []topicRef {
	var refs []topicRef
	for mi, mod := range modules {
		mid := program.GetString(mod, "id", "")
		topicsAny, _ := program.AsArray(mod["topics"])
		if len(topicsAny) == 0 {
			t := program.Object{
				"id":         "T" + itoa(mi+1) + ".1",
				"title":      "Тема",
				"weight":     1.0,
				"kinds":      program.Array{"lecture", "practice"},
				"outcomeIds": program.Array{},
			}
			topicsAny = program.Array{t}
			mod["topics"] = topicsAny
		} else {
			// Ensure array holds objects we can mutate.
			fixed := make(program.Array, len(topicsAny))
			for i, t := range topicsAny {
				if to, ok := program.AsObject(t); ok {
					fixed[i] = to
				} else {
					fixed[i] = t
				}
			}
			mod["topics"] = fixed
			topicsAny = fixed
		}
		modWeight := program.GetFloat(mod, "weight", 0)
		if modWeight <= 0 {
			modWeight = 1
		}
		for ti, t := range topicsAny {
			to, ok := program.AsObject(t)
			if !ok {
				continue
			}
			tid := program.GetString(to, "id", "")
			tw := program.GetFloat(to, "weight", 0)
			if tw <= 0 {
				tw = 1
			}
			refs = append(refs, topicRef{
				modIdx:   mi,
				topicIdx: ti,
				weight:   modWeight * tw,
				locked:   lockedIDs[tid] || lockedIDs[mid],
			})
		}
	}
	return refs
}

func topicAt(modules []program.Object, ref topicRef) program.Object {
	topicsAny, _ := program.AsArray(modules[ref.modIdx]["topics"])
	to, _ := program.AsObject(topicsAny[ref.topicIdx])
	return to
}

func rollupModuleHours(mod program.Object, lockedIDs map[string]bool) {
	mid := program.GetString(mod, "id", "")
	topicsAny, _ := program.AsArray(mod["topics"])
	var modHours program.Hours
	for _, t := range topicsAny {
		to, ok := program.AsObject(t)
		if !ok {
			continue
		}
		th := program.HoursFromObject(program.EnsureObject(to, "hours"))
		modHours.Lecture += th.Lecture
		modHours.Practice += th.Practice
		modHours.Lab += th.Lab
		modHours.SelfStudy += th.SelfStudy
	}
	modAssessment := 0.0
	if lockedIDs[mid] {
		modAssessment = program.HoursFromObject(program.EnsureObject(mod, "hours")).Assessment
	}
	modHours.Assessment = modAssessment
	modHours.Normalize()
	mod["hours"] = modHours.ToObject()
	if _, ok := mod["assessmentForm"]; !ok {
		mod["assessmentForm"] = "нет"
	}
}

func recomputeTotals(curriculum program.Object, modules []program.Object, fa program.Object) {
	var rows []program.Hours
	for _, mod := range modules {
		rows = append(rows, program.HoursFromObject(program.EnsureObject(mod, "hours")))
	}
	rows = append(rows, program.HoursFromObject(program.EnsureObject(fa, "hours")))
	sum := program.SumHours(rows)
	totals := program.Hours{
		Lecture:    sum.Lecture,
		Practice:   sum.Practice,
		Lab:        sum.Lab,
		SelfStudy:  sum.SelfStudy,
		Assessment: sum.Assessment,
	}
	totals.Normalize()
	curriculum["totals"] = totals.ToObject()
}

func reserveAttestation(total float64) float64 {
	v := math.Round(total*0.1*2) / 2
	if v < 1 {
		v = 1
	}
	if v > 4 {
		v = 4
	}
	if v > total-0.5 {
		v = math.Max(0.5, total-0.5)
	}
	return roundHalf(v)
}

func splitTopicHours(topic program.Object, total float64) program.Hours {
	total = roundHalf(total)
	if total <= 0 {
		h := program.Hours{}
		h.Normalize()
		return h
	}
	kinds := kindSet(topic)
	hint := program.GetFloat(topic, "practiceShareHint", 0.6)
	if hint < 0 {
		hint = 0
	}
	if hint > 1 {
		hint = 1
	}

	onlySelf := len(kinds) == 1 && kinds["selfStudy"]
	if len(kinds) == 0 {
		kinds = map[string]bool{"lecture": true, "practice": true}
	}
	if onlySelf {
		h := program.Hours{SelfStudy: total}
		h.Normalize()
		return h
	}

	half := program.Half(total)
	hasContact := kinds["lecture"] || kinds["practice"] || kinds["lab"]
	selfHalf := 0
	if kinds["selfStudy"] && hasContact {
		selfHalf = half / 5
		if selfHalf == 0 && half > 1 {
			selfHalf = 1
		}
	} else if kinds["selfStudy"] && !hasContact {
		selfHalf = half
	}
	contactHalf := half - selfHalf

	practiceHalf := 0
	labHalf := 0
	lectureHalf := 0

	if contactHalf > 0 {
		practiceTarget := 0.0
		if kinds["practice"] || kinds["lab"] {
			practiceTarget = hint
		}
		practicalHalf := int(math.Round(float64(contactHalf) * practiceTarget))
		if practicalHalf > contactHalf {
			practicalHalf = contactHalf
		}
		switch {
		case kinds["lab"] && kinds["practice"]:
			labHalf = practicalHalf / 2
			practiceHalf = practicalHalf - labHalf
		case kinds["lab"]:
			labHalf = practicalHalf
		case kinds["practice"]:
			practiceHalf = practicalHalf
		}
		if kinds["lecture"] {
			lectureHalf = contactHalf - practiceHalf - labHalf
		} else {
			rem := contactHalf - practiceHalf - labHalf
			switch {
			case kinds["practice"]:
				practiceHalf += rem
			case kinds["lab"]:
				labHalf += rem
			default:
				selfHalf += rem
			}
		}
	}

	h := program.Hours{
		Lecture:   program.FromHalf(lectureHalf),
		Practice:  program.FromHalf(practiceHalf),
		Lab:       program.FromHalf(labHalf),
		SelfStudy: program.FromHalf(selfHalf),
	}
	h.Normalize()
	if program.Half(h.Total) != program.Half(total) {
		diff := program.Half(total) - program.Half(h.Total)
		switch {
		case kinds["practice"]:
			h.Practice = program.FromHalf(program.Half(h.Practice) + diff)
		case kinds["lecture"]:
			h.Lecture = program.FromHalf(program.Half(h.Lecture) + diff)
		default:
			h.SelfStudy = program.FromHalf(program.Half(h.SelfStudy) + diff)
		}
		h.Normalize()
	}
	return h
}

func kindSet(topic program.Object) map[string]bool {
	out := map[string]bool{}
	arr, ok := program.AsArray(topic["kinds"])
	if !ok {
		return out
	}
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out[s] = true
		}
	}
	return out
}

// largestRemainder distributes totalHalf units by weights (Hamilton method).
func largestRemainder(weights []float64, totalHalf int) []int {
	n := len(weights)
	out := make([]int, n)
	if n == 0 || totalHalf <= 0 {
		return out
	}
	sumW := 0.0
	for _, w := range weights {
		if w > 0 {
			sumW += w
		}
	}
	if sumW <= 0 {
		base := totalHalf / n
		rem := totalHalf % n
		for i := range out {
			out[i] = base
			if i < rem {
				out[i]++
			}
		}
		return out
	}

	type frac struct {
		i    int
		rest float64
	}
	assigned := 0
	fracs := make([]frac, 0, n)
	for i, w := range weights {
		if w < 0 {
			w = 0
		}
		exact := float64(totalHalf) * (w / sumW)
		floor := int(math.Floor(exact))
		out[i] = floor
		assigned += floor
		fracs = append(fracs, frac{i: i, rest: exact - float64(floor)})
	}
	rem := totalHalf - assigned
	sort.SliceStable(fracs, func(a, b int) bool {
		if fracs[a].rest == fracs[b].rest {
			return fracs[a].i < fracs[b].i
		}
		return fracs[a].rest > fracs[b].rest
	})
	for k := 0; k < rem && k < len(fracs); k++ {
		out[fracs[k].i]++
	}
	return out
}

func roundHalf(x float64) float64 {
	return math.Round(x*2) / 2
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
