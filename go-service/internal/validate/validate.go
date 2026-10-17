package validate

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/danjos91/ALTDA/go-service/internal/program"
)

var hourKeys = []string{"lecture", "practice", "lab", "selfStudy", "assessment"}

var activityHoursKey = map[string]string{
	"lecture":   "lecture",
	"practice":  "practice",
	"lab":       "lab",
	"selfStudy": "selfStudy",
	"control":   "assessment",
}

var statusOrder = []string{
	"intake", "clarifying", "skeleton_proposed", "skeleton_approved",
	"content_generating", "content_ready", "validated", "exported",
}

const (
	minAssessmentsPerOutcome = 2
	minutesPerAcademicHour   = 45
	weightTolerance          = 0.01
)

// Check is one validation check (matches Python check() shape / schema).
type Check struct {
	RuleID     string   `json:"ruleId"`
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Passed     bool     `json:"passed"`
	Message    string   `json:"message"`
	Path       string   `json:"path,omitempty"`
	RelatedIDs []string `json:"relatedIds,omitempty"`
}

// Issue is a failed check flattened for API responses.
type Issue struct {
	RuleID   string `json:"ruleId"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

func addCheck(checks *[]Check, ruleID, category string, ok bool, message, path string, related []string, severity string) {
	if related == nil {
		related = []string{}
	}
	if severity == "" {
		severity = "error"
	}
	*checks = append(*checks, Check{
		RuleID:     ruleID,
		Category:   category,
		Severity:   severity,
		Passed:     ok,
		Message:    message,
		Path:       path,
		RelatedIDs: related,
	})
}

func statusAtLeast(prog program.Object, status string) bool {
	cur := program.GetString(prog, "status", "intake")
	return indexOf(statusOrder, cur) >= indexOf(statusOrder, status)
}

func indexOf(list []string, v string) int {
	for i, s := range list {
		if s == v {
			return i
		}
	}
	return -1
}

func checkRow(checks *[]Check, h program.Hours, path, label string) {
	contact := h.Lecture + h.Practice + h.Lab
	addCheck(checks, "HOURS_CONTACT", "hours",
		program.Half(h.Contact) == program.Half(contact),
		fmt.Sprintf("%s: contact=%v, expected %v", label, h.Contact, contact), path, nil, "error")
	total := contact + h.SelfStudy + h.Assessment
	addCheck(checks, "HOURS_ROW_SUM", "hours",
		program.Half(h.Total) == program.Half(total),
		fmt.Sprintf("%s: total=%v, expected %v", label, h.Total, total), path, nil, "error")
}

func allTopics(cur program.Object) []program.Object {
	var out []program.Object
	mods, _ := program.AsArray(cur["modules"])
	for _, m := range mods {
		mo, ok := program.AsObject(m)
		if !ok {
			continue
		}
		topics, _ := program.AsArray(mo["topics"])
		for _, t := range topics {
			if to, ok := program.AsObject(t); ok {
				out = append(out, to)
			}
		}
	}
	return out
}

func outcomeIDsOf(competency program.Object) []string {
	var ids []string
	for _, key := range []string{"knowledge", "skills", "abilities"} {
		arr, _ := program.AsArray(competency[key])
		for _, o := range arr {
			if oo, ok := program.AsObject(o); ok {
				if id := program.GetString(oo, "id", ""); id != "" {
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

func validateHours(prog program.Object, checks *[]Check) {
	cur := program.Nested(prog, "curriculum")
	if cur == nil {
		return
	}
	mods, _ := program.AsArray(cur["modules"])
	for mi, m := range mods {
		mo, ok := program.AsObject(m)
		if !ok {
			continue
		}
		mpath := fmt.Sprintf("/curriculum/modules/%d", mi)
		mh := program.HoursFromObject(program.EnsureObject(mo, "hours"))
		checkRow(checks, mh, mpath+"/hours", program.GetString(mo, "id", ""))
		topics, _ := program.AsArray(mo["topics"])
		var topicHours []program.Hours
		for ti, t := range topics {
			to, ok := program.AsObject(t)
			if !ok {
				continue
			}
			th := program.HoursFromObject(program.EnsureObject(to, "hours"))
			checkRow(checks, th, fmt.Sprintf("%s/topics/%d/hours", mpath, ti), program.GetString(to, "id", ""))
			topicHours = append(topicHours, th)
		}
		s := program.SumHours(topicHours)
		s.Assessment = mh.Assessment
		s.Total = s.Lecture + s.Practice + s.Lab + s.SelfStudy + s.Assessment
		addCheck(checks, "HOURS_TOPICS_EQ_MODULE", "hours",
			program.EqualHalf(s, mh, append(hourKeys, "total")),
			fmt.Sprintf("%s: sum of topics != module", program.GetString(mo, "id", "")),
			mpath+"/hours", []string{program.GetString(mo, "id", "")}, "error")
	}

	fa := program.Nested(prog, "finalAttestation")
	fah := program.HoursFromObject(program.EnsureObject(fa, "hours"))
	checkRow(checks, fah, "/finalAttestation/hours", "Final attestation")

	var rows []program.Hours
	for _, m := range mods {
		if mo, ok := program.AsObject(m); ok {
			rows = append(rows, program.HoursFromObject(program.EnsureObject(mo, "hours")))
		}
	}
	rows = append(rows, fah)
	grand := program.SumHours(rows)
	totals := program.HoursFromObject(program.EnsureObject(cur, "totals"))
	addCheck(checks, "HOURS_TOTALS_FIELD", "hours",
		program.EqualHalf(grand, totals, program.AllHourKeys),
		fmt.Sprintf("declared totals != computed"),
		"/curriculum/totals", nil, "error")
	requested := program.GetFloat(program.Nested(prog, "header"), "totalHours", 0)
	addCheck(checks, "HOURS_EQ_REQUESTED", "hours",
		program.Half(grand.Total) == program.Half(requested),
		fmt.Sprintf("Sum %v h != requested %v h", grand.Total, requested),
		"/header/totalHours", nil, "error")

	params := program.Nested(prog, "request", "params")
	if params != nil {
		if v, ok := params["minPracticeShare"]; ok && v != nil {
			minShare, ok := v.(float64)
			if ok && grand.Total > 0 {
				share := (grand.Practice + grand.Lab) / grand.Total
				addCheck(checks, "HOURS_PRACTICE_SHARE", "hours",
					share+1e-9 >= minShare,
					fmt.Sprintf("Practice share %.0f%% < required %.0f%%", share*100, minShare*100),
					"/curriculum/totals", nil, "error")
			}
		}
	}

	for mi, m := range mods {
		mo, ok := program.AsObject(m)
		if !ok {
			continue
		}
		topics, _ := program.AsArray(mo["topics"])
		for ti, t := range topics {
			to, ok := program.AsObject(t)
			if !ok {
				continue
			}
			acts, ok := program.AsArray(to["activities"])
			if !ok || acts == nil {
				continue
			}
			byKey := map[string]float64{}
			for _, k := range hourKeys {
				byKey[k] = 0
			}
			for _, a := range acts {
				ao, ok := program.AsObject(a)
				if !ok {
					continue
				}
				kind := program.GetString(ao, "kind", "")
				hk, ok := activityHoursKey[kind]
				if !ok {
					continue
				}
				byKey[hk] += program.GetFloat(ao, "hours", 0)
			}
			th := program.HoursFromObject(program.EnsureObject(to, "hours"))
			byHours := program.Hours{
				Lecture: byKey["lecture"], Practice: byKey["practice"], Lab: byKey["lab"],
				SelfStudy: byKey["selfStudy"], Assessment: byKey["assessment"],
			}
			addCheck(checks, "HOURS_ACTIVITIES_EQ_TOPIC", "hours",
				program.EqualHalf(byHours, th, hourKeys),
				fmt.Sprintf("%s: activities != topic hours", program.GetString(to, "id", "")),
				fmt.Sprintf("/curriculum/modules/%d/topics/%d/activities", mi, ti),
				[]string{program.GetString(to, "id", "")}, "error")
		}
	}
}

func validateCalendar(prog program.Object, checks *[]Check, topicIDs map[string]bool) {
	cal := program.Nested(prog, "calendar")
	if cal == nil {
		return
	}
	sessions, _ := program.AsArray(cal["sessions"])
	numbers := make([]int, 0, len(sessions))
	for _, s := range sessions {
		so, ok := program.AsObject(s)
		if !ok {
			continue
		}
		numbers = append(numbers, int(program.GetFloat(so, "number", 0)))
	}
	okNums := true
	for i, n := range numbers {
		if n != i+1 {
			okNums = false
			break
		}
	}
	addCheck(checks, "CALENDAR_SESSION_NUMBERS", "consistency", okNums,
		fmt.Sprintf("Session numbers must be 1..%d in order, got %v", len(sessions), numbers),
		"/calendar/sessions", nil, "error")

	badSet := map[string]bool{}
	for _, s := range sessions {
		so, ok := program.AsObject(s)
		if !ok {
			continue
		}
		ids, _ := program.AsArray(so["topicIds"])
		for _, id := range ids {
			if sid, ok := id.(string); ok && !topicIDs[sid] {
				badSet[sid] = true
			}
		}
	}
	bad := sortedKeys(badSet)
	addCheck(checks, "CONSISTENCY_DANGLING_TOPIC", "consistency", len(bad) == 0,
		fmt.Sprintf("Calendar references non-existent topics: %v", bad),
		"/calendar/sessions", bad, "error")

	totals := program.HoursFromObject(program.EnsureObject(program.Nested(prog, "curriculum"), "totals"))
	scheduled := 0.0
	for _, s := range sessions {
		if so, ok := program.AsObject(s); ok {
			scheduled += program.GetFloat(so, "hours", 0)
		}
	}
	expected := totals.Total - totals.SelfStudy
	addCheck(checks, "CALENDAR_HOURS", "hours",
		program.Half(scheduled) == program.Half(expected),
		fmt.Sprintf("Sessions add up to %v h, expected %v h (total minus self-study)", scheduled, expected),
		"/calendar/sessions", nil, "warning")
}

func validateCoverage(prog program.Object, checks *[]Check, outcomes map[string]bool) {
	cur := program.Nested(prog, "curriculum")
	taught := map[string]bool{}
	for _, t := range allTopics(cur) {
		ids, _ := program.AsArray(t["outcomeIds"])
		for _, id := range ids {
			if s, ok := id.(string); ok {
				taught[s] = true
			}
		}
	}

	tools := map[string]int{}
	for oid := range outcomes {
		tools[oid] = 0
	}
	assessment := program.Nested(prog, "assessment")
	var uses [][]string
	if assessment != nil {
		tests, _ := program.AsArray(assessment["tests"])
		for _, test := range tests {
			to, ok := program.AsObject(test)
			if !ok {
				continue
			}
			qs, _ := program.AsArray(to["questions"])
			for _, q := range qs {
				qo, ok := program.AsObject(q)
				if !ok {
					continue
				}
				uses = append(uses, stringSlice(qo["outcomeIds"]))
			}
		}
		tasks, _ := program.AsArray(assessment["seminarTasks"])
		for _, s := range tasks {
			if so, ok := program.AsObject(s); ok {
				uses = append(uses, stringSlice(so["outcomeIds"]))
			}
		}
	}
	fa := program.Nested(prog, "finalAttestation")
	uses = append(uses, stringSlice(fa["outcomeIds"]))
	for _, ids := range uses {
		seen := map[string]bool{}
		for _, oid := range ids {
			if outcomes[oid] && !seen[oid] {
				tools[oid]++
				seen[oid] = true
			}
		}
	}

	assessedStage := statusAtLeast(prog, "content_ready") || assessment != nil
	oids := sortedKeys(outcomes)
	for _, oid := range oids {
		addCheck(checks, "COVERAGE_OUTCOME_TAUGHT", "coverage", taught[oid],
			fmt.Sprintf("%s does not appear in any topic", oid), "", []string{oid}, "error")
		if assessedStage {
			addCheck(checks, "COVERAGE_OUTCOME_ASSESSED", "coverage", tools[oid] >= minAssessmentsPerOutcome,
				fmt.Sprintf("%s has %d assessment tool(s), needs >= %d", oid, tools[oid], minAssessmentsPerOutcome),
				"", []string{oid}, "error")
		}
	}

	comps, _ := program.AsArray(prog["competencies"])
	for _, c := range comps {
		co, ok := program.AsObject(c)
		if !ok {
			continue
		}
		cid := program.GetString(co, "id", "")
		var bad []string
		for _, i := range outcomeIDsOf(co) {
			prefix := strings.Split(i, ".")[0]
			if prefix != cid {
				bad = append(bad, i)
			}
		}
		addCheck(checks, "CONSISTENCY_OUTCOME_PREFIX", "consistency", len(bad) == 0,
			fmt.Sprintf("%s: outcomes with a foreign prefix: %v", cid, bad), "", bad, "error")
	}

	referenced := map[string]bool{}
	for k := range taught {
		referenced[k] = true
	}
	for _, ids := range uses {
		for _, id := range ids {
			referenced[id] = true
		}
	}
	lecs, _ := program.AsArray(prog["lectures"])
	for _, lec := range lecs {
		if lo, ok := program.AsObject(lec); ok {
			for _, id := range stringSlice(lo["outcomeIds"]) {
				referenced[id] = true
			}
		}
	}
	var dangling []string
	for id := range referenced {
		if !outcomes[id] {
			dangling = append(dangling, id)
		}
	}
	sort.Strings(dangling)
	addCheck(checks, "CONSISTENCY_DANGLING_OUTCOME", "consistency", len(dangling) == 0,
		fmt.Sprintf("Non-existent outcomes referenced: %v", dangling), "", dangling, "error")
}

func validateReferences(prog program.Object, checks *[]Check, topicIDs map[string]bool) {
	cur := program.Nested(prog, "curriculum")
	assessment := program.Nested(prog, "assessment")
	topicRefs := map[string]bool{}

	lecs, _ := program.AsArray(prog["lectures"])
	for _, lec := range lecs {
		if lo, ok := program.AsObject(lec); ok {
			if tid := program.GetString(lo, "topicId", ""); tid != "" {
				topicRefs[tid] = true
			}
		}
	}
	if assessment != nil {
		for _, s := range program.EnsureArray(assessment, "seminarTasks") {
			if so, ok := program.AsObject(s); ok {
				if tid := program.GetString(so, "topicId", ""); tid != "" {
					topicRefs[tid] = true
				}
			}
		}
		for _, test := range program.EnsureArray(assessment, "tests") {
			to, ok := program.AsObject(test)
			if !ok {
				continue
			}
			for _, q := range program.EnsureArray(to, "questions") {
				qo, ok := program.AsObject(q)
				if !ok {
					continue
				}
				for _, tid := range stringSlice(qo["topicIds"]) {
					topicRefs[tid] = true
				}
			}
		}
	}
	for _, s := range program.EnsureArray(prog, "sources") {
		if so, ok := program.AsObject(s); ok {
			for _, tid := range stringSlice(so["topicIds"]) {
				topicRefs[tid] = true
			}
		}
	}
	for _, t := range program.EnsureArray(prog, "tools") {
		if to, ok := program.AsObject(t); ok {
			for _, tid := range stringSlice(to["usedInTopics"]) {
				topicRefs[tid] = true
			}
		}
	}
	for _, t := range allTopics(cur) {
		for _, p := range stringSlice(t["prerequisites"]) {
			topicRefs[p] = true
		}
	}
	badTopics := []string{}
	for tid := range topicRefs {
		if !topicIDs[tid] {
			badTopics = append(badTopics, tid)
		}
	}
	sort.Strings(badTopics)
	addCheck(checks, "CONSISTENCY_DANGLING_TOPIC", "consistency", len(badTopics) == 0,
		fmt.Sprintf("Non-existent topics referenced: %v", badTopics), "", badTopics, "error")

	position := map[string]int{}
	for i, t := range allTopics(cur) {
		position[program.GetString(t, "id", "")] = i
	}
	var late []string
	for _, t := range allTopics(cur) {
		tid := program.GetString(t, "id", "")
		for _, p := range stringSlice(t["prerequisites"]) {
			if pos, ok := position[p]; ok && pos >= position[tid] {
				late = append(late, fmt.Sprintf("%s <- %s", tid, p))
			}
		}
	}
	addCheck(checks, "ORDER_PREREQUISITES", "consistency", len(late) == 0,
		fmt.Sprintf("Prerequisites that do not come earlier: %v", late), "", late, "error")

	refIDs := map[string]bool{}
	req := program.Nested(prog, "request")
	if req != nil {
		for _, a := range program.EnsureArray(req, "attachments") {
			if ao, ok := program.AsObject(a); ok {
				refIDs[program.GetString(ao, "id", "")] = true
			}
		}
	}
	usedRefs := map[string]bool{}
	for _, t := range allTopics(cur) {
		for _, rid := range stringSlice(t["referenceIds"]) {
			usedRefs[rid] = true
		}
	}
	for _, lec := range lecs {
		if lo, ok := program.AsObject(lec); ok {
			for _, rid := range stringSlice(lo["referenceIds"]) {
				usedRefs[rid] = true
			}
		}
	}
	badRefs := []string{}
	for rid := range usedRefs {
		if !refIDs[rid] {
			badRefs = append(badRefs, rid)
		}
	}
	sort.Strings(badRefs)
	addCheck(checks, "CONSISTENCY_DANGLING_REFERENCE", "consistency", len(badRefs) == 0,
		fmt.Sprintf("Non-existent references cited: %v", badRefs), "", badRefs, "error")

	sourceIDs := map[string]bool{}
	for _, s := range program.EnsureArray(prog, "sources") {
		if so, ok := program.AsObject(s); ok {
			sourceIDs[program.GetString(so, "id", "")] = true
		}
	}
	usedSrc := map[string]bool{}
	for _, sid := range stringSlice(prog["normativeBasis"]) {
		usedSrc[sid] = true
	}
	for _, lec := range lecs {
		if lo, ok := program.AsObject(lec); ok {
			for _, sid := range stringSlice(lo["sourceIds"]) {
				usedSrc[sid] = true
			}
		}
	}
	badSrc := []string{}
	for sid := range usedSrc {
		if !sourceIDs[sid] {
			badSrc = append(badSrc, sid)
		}
	}
	sort.Strings(badSrc)
	addCheck(checks, "CONSISTENCY_DANGLING_SOURCE", "consistency", len(badSrc) == 0,
		fmt.Sprintf("Non-existent sources cited: %v", badSrc), "", badSrc, "error")

	if program.GetString(prog, "programType", "") == "DPP_PK" {
		standards := map[string]map[string]bool{}
		dpp := program.Nested(prog, "dpp")
		if dpp != nil {
			for _, p := range program.EnsureArray(dpp, "profStandards") {
				po, ok := program.AsObject(p)
				if !ok {
					continue
				}
				ref := program.GetString(po, "registryRef", "")
				funcs := map[string]bool{}
				for _, f := range program.EnsureArray(po, "laborFunctions") {
					if fo, ok := program.AsObject(f); ok {
						funcs[program.GetString(fo, "code", "")] = true
					}
				}
				standards[ref] = funcs
			}
		}
		var badLinks []string
		for _, c := range program.EnsureArray(prog, "competencies") {
			co, ok := program.AsObject(c)
			if !ok {
				continue
			}
			cid := program.GetString(co, "id", "")
			for _, link := range program.EnsureArray(co, "profStandardLinks") {
				lo, ok := program.AsObject(link)
				if !ok {
					continue
				}
				ref := program.GetString(lo, "registryRef", "")
				code := program.GetString(lo, "laborFunctionCode", "")
				funcs, ok := standards[ref]
				if !ok || (len(funcs) > 0 && !funcs[code]) {
					badLinks = append(badLinks, fmt.Sprintf("%s: %s/%s", cid, ref, code))
				}
			}
		}
		addCheck(checks, "CONSISTENCY_PROFSTANDARD_LINK", "consistency", len(badLinks) == 0,
			fmt.Sprintf("Competency links not found in dpp.profStandards: %v", badLinks), "", badLinks, "error")
	}
}

func validateContent(prog program.Object, checks *[]Check) {
	cur := program.Nested(prog, "curriculum")
	assessment := program.Nested(prog, "assessment")
	topics := map[string]program.Object{}
	for _, t := range allTopics(cur) {
		topics[program.GetString(t, "id", "")] = t
	}

	minutes := map[string]float64{}
	for _, lec := range program.EnsureArray(prog, "lectures") {
		lo, ok := program.AsObject(lec)
		if !ok {
			continue
		}
		tid := program.GetString(lo, "topicId", "")
		plan, _ := program.AsArray(lo["plan"])
		for _, s := range plan {
			if so, ok := program.AsObject(s); ok {
				minutes[tid] += program.GetFloat(so, "minutes", 0)
			}
		}
	}
	for tid, mins := range minutes {
		t, ok := topics[tid]
		if !ok {
			continue
		}
		th := program.HoursFromObject(program.EnsureObject(t, "hours"))
		expected := int(th.Lecture*minutesPerAcademicHour + 0.5)
		addCheck(checks, "LECTURE_MINUTES", "hours", int(mins+0.5) == expected,
			fmt.Sprintf("%s: lecture plans add up to %v min, expected %d min", tid, mins, expected),
			"", []string{tid}, "error")
	}

	if assessment != nil {
		for _, test := range program.EnsureArray(assessment, "tests") {
			to, ok := program.AsObject(test)
			if !ok {
				continue
			}
			for _, q := range program.EnsureArray(to, "questions") {
				qo, ok := program.AsObject(q)
				if !ok {
					continue
				}
				okKey, msg := testKeyOK(qo)
				addCheck(checks, "TEST_KEY", "consistency", okKey,
					fmt.Sprintf("%s/%s: %s", program.GetString(to, "id", ""), program.GetString(qo, "id", ""), msg),
					"", []string{program.GetString(qo, "id", "")}, "error")
			}
		}

		perTopic := map[string]float64{}
		for _, s := range program.EnsureArray(assessment, "seminarTasks") {
			so, ok := program.AsObject(s)
			if !ok {
				continue
			}
			total := 0.0
			for _, c := range program.EnsureArray(so, "criteria") {
				if co, ok := program.AsObject(c); ok {
					total += program.GetFloat(co, "maxPoints", 0)
				}
			}
			maxScore := program.GetFloat(so, "maxScore", 0)
			addCheck(checks, "TASK_MAXSCORE", "consistency", total == maxScore,
				fmt.Sprintf("%s: maxScore=%v, criteria add up to %v", program.GetString(so, "id", ""), maxScore, total),
				"", []string{program.GetString(so, "id", "")}, "error")
			tid := program.GetString(so, "topicId", "")
			perTopic[tid] += program.GetFloat(so, "durationHours", 0)
		}
		for tid, dur := range perTopic {
			t, ok := topics[tid]
			if !ok {
				continue
			}
			h := program.HoursFromObject(program.EnsureObject(t, "hours"))
			cap := h.Practice + h.Lab
			addCheck(checks, "TASK_DURATION", "hours", program.Half(dur) <= program.Half(cap),
				fmt.Sprintf("%s: tasks take %v h, topic has %v h of practice + lab", tid, dur, cap),
				"", []string{tid}, "error")
		}
	}
}

func testKeyOK(q program.Object) (bool, string) {
	t := program.GetString(q, "type", "")
	correct := stringSlice(q["correct"])
	if t == "short" {
		return true, "ok"
	}
	optionIDs := []string{}
	seen := map[string]bool{}
	for _, o := range program.EnsureArray(q, "options") {
		if oo, ok := program.AsObject(o); ok {
			id := program.GetString(oo, "id", "")
			if seen[id] {
				return false, "duplicate option ids"
			}
			seen[id] = true
			optionIDs = append(optionIDs, id)
		}
	}
	optSet := map[string]bool{}
	for _, id := range optionIDs {
		optSet[id] = true
	}
	var unknown []string
	for _, c := range correct {
		if !optSet[c] {
			unknown = append(unknown, c)
		}
	}
	if len(unknown) > 0 {
		return false, fmt.Sprintf("correct answers not among the options: %v", unknown)
	}
	if t == "single" || t == "truefalse" {
		return len(correct) == 1, fmt.Sprintf("%d correct answers (must be exactly 1)", len(correct))
	}
	if t == "multiple" {
		ok := 2 <= len(correct) && len(correct) < len(optionIDs)
		return ok, fmt.Sprintf("%d correct of %d options (need 2+, and not all)", len(correct), len(optionIDs))
	}
	return true, "ok"
}

// ValidateProgram ports tools/validate_reference.py::validate.
func ValidateProgram(prog program.Object) []Check {
	var checks []Check
	if !statusAtLeast(prog, "skeleton_proposed") {
		return checks
	}
	cur := program.Nested(prog, "curriculum")
	if cur == nil {
		return checks
	}
	outcomes := map[string]bool{}
	for _, c := range program.EnsureArray(prog, "competencies") {
		if co, ok := program.AsObject(c); ok {
			for _, id := range outcomeIDsOf(co) {
				outcomes[id] = true
			}
		}
	}
	topicIDs := map[string]bool{}
	for _, t := range allTopics(cur) {
		topicIDs[program.GetString(t, "id", "")] = true
	}

	validateHours(prog, &checks)
	validateCalendar(prog, &checks, topicIDs)
	validateCoverage(prog, &checks, outcomes)
	validateReferences(prog, &checks, topicIDs)
	validateContent(prog, &checks)

	for _, t := range program.EnsureArray(prog, "tools") {
		to, ok := program.AsObject(t)
		if !ok {
			continue
		}
		if program.GetString(to, "status", "") == "outdated" {
			related := stringSlice(to["usedInTopics"])
			addCheck(&checks, "TOOL_OUTDATED", "consistency", false,
				fmt.Sprintf("%s %s is outdated (current: %v); topics: %v",
					program.GetString(to, "name", ""), program.GetString(to, "version", ""),
					to["latestVersion"], related),
				"", related, "warning")
		}
	}

	for i, s := range program.EnsureArray(prog, "sources") {
		so, ok := program.AsObject(s)
		if !ok {
			continue
		}
		verified := program.GetBool(so, "verified", false)
		addCheck(&checks, "SOURCE_UNVERIFIED", "sources", verified,
			fmt.Sprintf("Source %s: unverified", program.GetString(so, "id", "")),
			fmt.Sprintf("/sources/%d", i), []string{program.GetString(so, "id", "")}, "warning")
	}

	for _, sec := range program.EnsureArray(prog, "templateSections") {
		so, ok := program.AsObject(sec)
		if !ok {
			continue
		}
		addCheck(&checks, "SECTION_MISSING", "completeness",
			program.GetString(so, "status", "") != "missing",
			fmt.Sprintf("Empty template section: %s", program.GetString(so, "title", "")),
			"", []string{program.GetString(so, "key", "")}, "error")
	}

	return checks
}

// ValidateSkeletonResponse ports validate_skeleton_response.
func ValidateSkeletonResponse(resp program.Object) []Check {
	var checks []Check
	variants, _ := program.AsArray(resp["variants"])
	for _, v := range variants {
		vo, ok := program.AsObject(v)
		if !ok {
			continue
		}
		key := fmt.Sprintf("variant %s", program.GetString(vo, "key", "?"))
		outcomes := map[string]bool{}
		for _, c := range program.EnsureArray(vo, "competencies") {
			if co, ok := program.AsObject(c); ok {
				for _, id := range outcomeIDsOf(co) {
					outcomes[id] = true
				}
			}
		}
		topicPos := map[string]int{}
		idx := 0
		for _, m := range program.EnsureArray(vo, "modules") {
			mo, ok := program.AsObject(m)
			if !ok {
				continue
			}
			for _, t := range program.EnsureArray(mo, "topics") {
				if to, ok := program.AsObject(t); ok {
					topicPos[program.GetString(to, "id", "")] = idx
					idx++
				}
			}
		}
		w := 0.0
		for _, m := range program.EnsureArray(vo, "modules") {
			if mo, ok := program.AsObject(m); ok {
				w += program.GetFloat(mo, "weight", 0)
			}
		}
		addCheck(&checks, "SKELETON_MODULE_WEIGHTS", "hours", abs(w-1) <= weightTolerance,
			fmt.Sprintf("%s: module weights add up to %.3f, expected 1", key, w),
			"", []string{program.GetString(vo, "key", "")}, "error")
		for _, m := range program.EnsureArray(vo, "modules") {
			mo, ok := program.AsObject(m)
			if !ok {
				continue
			}
			tw := 0.0
			for _, t := range program.EnsureArray(mo, "topics") {
				if to, ok := program.AsObject(t); ok {
					tw += program.GetFloat(to, "weight", 0)
				}
			}
			addCheck(&checks, "SKELETON_TOPIC_WEIGHTS", "hours", abs(tw-1) <= weightTolerance,
				fmt.Sprintf("%s/%s: topic weights add up to %.3f, expected 1", key, program.GetString(mo, "id", ""), tw),
				"", []string{program.GetString(mo, "id", "")}, "error")
		}
		used := map[string]bool{}
		for _, m := range program.EnsureArray(vo, "modules") {
			mo, ok := program.AsObject(m)
			if !ok {
				continue
			}
			for _, t := range program.EnsureArray(mo, "topics") {
				if to, ok := program.AsObject(t); ok {
					for _, oid := range stringSlice(to["outcomeIds"]) {
						used[oid] = true
					}
				}
			}
		}
		unknown := []string{}
		for oid := range used {
			if !outcomes[oid] {
				unknown = append(unknown, oid)
			}
		}
		sort.Strings(unknown)
		addCheck(&checks, "SKELETON_DANGLING_OUTCOME", "consistency", len(unknown) == 0,
			fmt.Sprintf("%s: unknown outcomes %v", key, unknown), "", unknown, "error")
		missing := []string{}
		for oid := range outcomes {
			if !used[oid] {
				missing = append(missing, oid)
			}
		}
		sort.Strings(missing)
		addCheck(&checks, "COVERAGE_OUTCOME_TAUGHT", "coverage", len(missing) == 0,
			fmt.Sprintf("%s: outcomes not in any topic %v", key, missing), "", missing, "error")

		var late []string
		for _, m := range program.EnsureArray(vo, "modules") {
			mo, ok := program.AsObject(m)
			if !ok {
				continue
			}
			for _, t := range program.EnsureArray(mo, "topics") {
				to, ok := program.AsObject(t)
				if !ok {
					continue
				}
				tid := program.GetString(to, "id", "")
				for _, p := range stringSlice(to["prerequisites"]) {
					pos, ok := topicPos[p]
					if !ok || pos >= topicPos[tid] {
						late = append(late, fmt.Sprintf("%s <- %s", tid, p))
					}
				}
			}
		}
		addCheck(&checks, "ORDER_PREREQUISITES", "consistency", len(late) == 0,
			fmt.Sprintf("%s: bad prerequisites %v", key, late), "", late, "error")
	}
	return checks
}

// BuildReport builds validationReport + flat issues from checks.
func BuildReport(checks []Check, iterations int) (program.Object, []Issue) {
	errors, warnings, passed := 0, 0, 0
	unverified := 0
	var issues []Issue
	checkObjs := make(program.Array, 0, len(checks))
	for _, c := range checks {
		if c.Passed {
			passed++
		} else {
			if c.Severity == "error" {
				errors++
			} else if c.Severity == "warning" {
				warnings++
			}
			if c.RuleID == "SOURCE_UNVERIFIED" {
				unverified++
			}
			issues = append(issues, Issue{
				RuleID:   c.RuleID,
				Severity: c.Severity,
				Message:  c.Message,
				Path:     c.Path,
			})
		}
		obj := program.Object{
			"ruleId":     c.RuleID,
			"category":   c.Category,
			"severity":   c.Severity,
			"passed":     c.Passed,
			"message":    c.Message,
			"relatedIds": c.RelatedIDs,
		}
		if c.Path != "" {
			obj["path"] = c.Path
		}
		checkObjs = append(checkObjs, obj)
	}
	if issues == nil {
		issues = []Issue{}
	}
	report := program.Object{
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"iterations":  iterations,
		"summary": program.Object{
			"errors":             errors,
			"warnings":           warnings,
			"passed":             passed,
			"unverifiedSources":  unverified,
		},
		"checks": checkObjs,
	}
	return report, issues
}

func stringSlice(v any) []string {
	arr, ok := program.AsArray(v)
	if !ok || arr == nil {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
