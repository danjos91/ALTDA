package skeleton

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	allocateservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocate"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

func (s *service) Regenerate(ctx context.Context, p *program.Program, in RegenerateParams) (*RegenerateResult, error) {
	root := &p.V

	// Before the plan exists the variants live outside the program: offer new ones.
	if root.Curriculum == nil || len(root.Curriculum.V.Modules) == 0 {
		variants, err := s.generate(ctx, p, in.Comment)
		if err != nil {
			return nil, err
		}
		return &RegenerateResult{Variants: variants, ChangedPaths: []string{}}, nil
	}

	return s.regenerateModules(ctx, p, in)
}

func (s *service) regenerateModules(ctx context.Context, p *program.Program, in RegenerateParams) (*RegenerateResult, error) {
	root := &p.V

	req, err := requestFrom(p)
	if err != nil {
		return nil, err
	}
	req.Comment = in.Comment

	targets, explicit, err := resolveModules(root, in)
	if err != nil {
		return nil, err
	}

	// Locked modules: refuse when asked for by name, skip when merely inferred.
	var toRebuild []int
	for _, t := range targets {
		if in.RespectLocks && moduleIsLocked(&root.Curriculum.V.Modules[t.index].V) {
			if explicit {
				return nil, fmt.Errorf("%w: %s", ErrNodeLocked, t.asked)
			}
			continue
		}
		toRebuild = append(toRebuild, t.index)
	}
	if len(toRebuild) == 0 {
		return &RegenerateResult{Program: p, ChangedPaths: []string{}}, nil
	}

	before := hoursByPath(root)
	modules := root.Curriculum.V.Modules

	ids := make([]string, len(toRebuild))
	for i, index := range toRebuild {
		ids[i] = modules[index].V.ID
	}

	outcomes := outcomeIDsOf(root)
	proposed, err := s.regenerator.Regenerate(ctx, RegenerateInput{
		Request:  req,
		Comment:  in.Comment,
		Outcomes: outcomes,
		Current:  currentStructure(root),
		Targets:  ids,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoValidVariant, err)
	}
	if err := checkProposal(root, ids, proposed, outcomes); err != nil {
		return nil, err
	}

	replaced := map[string]bool{} // topics that no longer exist as they were
	for _, replacement := range proposed {
		for i := range modules {
			if modules[i].V.ID != replacement.ID {
				continue
			}
			for _, old := range modules[i].V.Topics {
				replaced[old.V.ID] = true
			}
			applyModule(&modules[i].V, replacement)
		}
	}
	dropContent(root, replaced)

	if err := s.reallocate.Allocate(ctx, p); err != nil {
		if errors.Is(err, allocateservice.ErrInvalidProgram) {
			return nil, fmt.Errorf("%w: %v", ErrInvalidProgram, err)
		}
		return nil, err
	}

	return &RegenerateResult{Program: p, ChangedPaths: changedPaths(root, ids, before)}, nil
}

type moduleTarget struct {
	index int
	asked string // the ID the caller used for it
}

var (
	moduleIDRe   = regexp.MustCompile(`^M\d+$`)
	topicIDRe    = regexp.MustCompile(`^T(\d+)\.\d+$`)
	mentionRe    = regexp.MustCompile(`(?i)\b(M\d+|T\d+\.\d+)\b`)
	moduleWordRe = regexp.MustCompile(`(?i)модул\p{L}*\s*(\d+)`)
	topicWordRe  = regexp.MustCompile(`(?i)тем\p{L}*\s*(\d+)\.(\d+)`)
)

// resolveModules turns node IDs into modules. A topic means its module. With
// no IDs, modules mentioned in the comment ("модуль 2", "тема 2.3", "M2") are
// taken, and with none of those, every module. explicit reports whether the
// caller named them.
func resolveModules(root *program.Root, in RegenerateParams) ([]moduleTarget, bool, error) {
	modules := root.Curriculum.V.Modules
	indexOf := func(moduleID string) int {
		for i := range modules {
			if modules[i].V.ID == moduleID {
				return i
			}
		}
		return -1
	}
	owner := func(topicID string) int {
		for i := range modules {
			for _, t := range modules[i].V.Topics {
				if t.V.ID == topicID {
					return i
				}
			}
		}
		return -1
	}

	var targets []moduleTarget
	add := func(index int, asked string) {
		for _, t := range targets {
			if t.index == index {
				return
			}
		}
		targets = append(targets, moduleTarget{index: index, asked: asked})
	}

	if len(in.NodeIDs) > 0 {
		for _, id := range in.NodeIDs {
			index := -1
			switch {
			case moduleIDRe.MatchString(id):
				index = indexOf(id)
			case topicIDRe.MatchString(id):
				index = owner(id)
			}
			if index < 0 {
				return nil, true, fmt.Errorf("%w: %q", ErrUnknownNode, id)
			}
			add(index, id)
		}
	} else {
		for _, m := range mentionRe.FindAllString(in.Comment, -1) {
			if i := mentioned(m, indexOf, owner); i >= 0 {
				add(i, m)
			}
		}
		for _, m := range moduleWordRe.FindAllStringSubmatch(in.Comment, -1) {
			if i := indexOf("M" + m[1]); i >= 0 {
				add(i, "M"+m[1])
			}
		}
		for _, m := range topicWordRe.FindAllStringSubmatch(in.Comment, -1) {
			if i := owner("T" + m[1] + "." + m[2]); i >= 0 {
				add(i, "T"+m[1]+"."+m[2])
			}
		}
		if len(targets) == 0 {
			for i := range modules {
				add(i, modules[i].V.ID)
			}
		}
	}

	sort.Slice(targets, func(a, b int) bool { return targets[a].index < targets[b].index })
	return targets, len(in.NodeIDs) > 0, nil
}

func mentioned(id string, indexOf func(string) int, owner func(string) int) int {
	id = upperFirst(id)
	if moduleIDRe.MatchString(id) {
		return indexOf(id)
	}
	return owner(id)
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}

// moduleIsLocked: a module is locked if it, or any of its topics, is. A module
// with a locked topic cannot be rebuilt without losing that topic.
func moduleIsLocked(m *program.Module) bool {
	if m.IsLocked() {
		return true
	}
	for i := range m.Topics {
		if m.Topics[i].V.IsLocked() {
			return true
		}
	}
	return false
}

func outcomeIDsOf(root *program.Root) []string {
	var ids []string
	for _, c := range root.Competencies {
		ids = append(ids, c.V.OutcomeIDs()...)
	}
	return ids
}

// currentStructure is the module and topic skeleton, for the regenerator's context.
func currentStructure(root *program.Root) []skeleton.Module {
	var out []skeleton.Module
	for _, m := range root.Curriculum.V.Modules {
		module := skeleton.Module{ID: m.V.ID, Title: m.V.Title}
		if m.V.Weight != nil {
			module.Weight = *m.V.Weight
		}
		for _, t := range m.V.Topics {
			module.Topics = append(module.Topics, skeleton.Topic{ID: t.V.ID, Title: t.V.Title})
		}
		out = append(out, module)
	}
	return out
}

// checkProposal refuses a replacement the program cannot take: wrong modules,
// no topics, outcomes that do not exist, or prerequisites that do not come first.
func checkProposal(root *program.Root, targets []string, proposed []skeleton.Module, outcomes []string) error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrNoValidVariant, fmt.Sprintf(format, args...))
	}

	got := map[string]bool{}
	for _, m := range proposed {
		got[m.ID] = true
		if !slices.Contains(targets, m.ID) {
			return bad("module %s was not requested", m.ID)
		}
		if len(m.Topics) == 0 {
			return bad("module %s has no topics", m.ID)
		}
	}
	for _, id := range targets {
		if !got[id] {
			return bad("module %s was not rebuilt", id)
		}
	}

	// Topic order across the whole curriculum, with the new modules in place.
	position := map[string]int{}
	n := 0
	for _, m := range root.Curriculum.V.Modules {
		topics := topicIDs(m.V)
		for _, replacement := range proposed {
			if replacement.ID == m.V.ID {
				topics = nil
				for _, t := range replacement.Topics {
					topics = append(topics, t.ID)
				}
			}
		}
		for _, id := range topics {
			if _, dup := position[id]; dup {
				return bad("topic %s appears twice", id)
			}
			position[id] = n
			n++
		}
	}

	for _, m := range proposed {
		for _, t := range m.Topics {
			if len(t.OutcomeIDs) == 0 {
				return bad("topic %s teaches no outcome", t.ID)
			}
			for _, id := range t.OutcomeIDs {
				if !slices.Contains(outcomes, id) {
					return bad("topic %s names the unknown outcome %s", t.ID, id)
				}
			}
			for _, pre := range t.Prerequisites {
				if at, known := position[pre]; !known || at >= position[t.ID] {
					return bad("topic %s has the prerequisite %s, which does not come earlier", t.ID, pre)
				}
			}
		}
	}
	return nil
}

func topicIDs(m program.Module) []string {
	ids := make([]string, len(m.Topics))
	for i, t := range m.Topics {
		ids[i] = t.V.ID
	}
	return ids
}

// applyModule writes the proposed structure into the module. Fields Go does
// not read (annotation, assessmentForm...) stay as they were. Each new topic
// starts with hours that only mark which kinds it has; the allocation that
// follows replaces them.
func applyModule(m *program.Module, proposed skeleton.Module) {
	m.Title = proposed.Title
	intro := proposed.IsIntro
	m.IsIntro = &intro
	weight := proposed.Weight
	m.Weight = &weight

	m.Topics = nil
	for _, t := range proposed.Topics {
		w := t.Weight
		m.Topics = append(m.Topics, program.Node[program.Topic]{V: program.Topic{
			ID:            t.ID,
			Title:         t.Title,
			Weight:        &w,
			OutcomeIDs:    t.OutcomeIDs,
			Prerequisites: t.Prerequisites,
			Hours:         seedHours(t.Kinds, t.PracticeShareHint),
		}})
	}
}

// seedHours encodes a topic's kinds, and its practice share, in placeholder
// hours. The allocator reads a topic's kinds from the columns in use, so this
// is how a freshly built topic tells it what it is made of.
func seedHours(kinds []string, practiceShare *float64) program.Hours {
	has := map[string]bool{}
	for _, k := range kinds {
		has[k] = true
	}
	practical := has["practice"] || has["lab"]

	share := 0.6
	if practiceShare != nil {
		share = *practiceShare
	}

	var h program.Hours
	switch {
	case has["lecture"] && practical:
		h.Lecture = 100 * (1 - share)
		spread(&h, has, 100*share)
	case has["lecture"]:
		h.Lecture = 100
	case practical:
		spread(&h, has, 100)
	}
	if has["selfStudy"] {
		if h.Lecture+h.Practice+h.Lab == 0 {
			h.SelfStudy = 100
		} else {
			h.SelfStudy = 10
		}
	}
	return h
}

func spread(h *program.Hours, has map[string]bool, amount float64) {
	switch {
	case has["practice"] && has["lab"]:
		h.Practice, h.Lab = amount/2, amount/2
	case has["lab"]:
		h.Lab = amount
	default:
		h.Practice = amount
	}
}

// dropContent removes what the content step wrote for topics that were just
// rebuilt: it described the old topics. The calendar is rebuilt by the next
// content run, and until then the validator flags it as out of date.
func dropContent(root *program.Root, topics map[string]bool) {
	if root.Lectures != nil {
		kept := []program.Node[program.Lecture]{}
		for _, l := range root.Lectures {
			if !topics[l.V.TopicID] {
				kept = append(kept, l)
			}
		}
		root.Lectures = kept
	}

	if root.Assessment == nil {
		return
	}
	assessment := &root.Assessment.V

	var tests []program.Node[program.Test]
	for _, test := range assessment.Tests {
		var questions []program.Node[program.Question]
		for _, q := range test.V.Questions {
			if len(q.V.TopicIDs) > 0 && topics[q.V.TopicIDs[0]] {
				continue
			}
			questions = append(questions, q)
		}
		if len(questions) == 0 && len(test.V.Questions) > 0 {
			continue // every question was about a rebuilt topic
		}
		test.V.Questions = questions
		tests = append(tests, test)
	}
	assessment.Tests = tests

	var tasks []program.Node[program.SeminarTask]
	for _, task := range assessment.SeminarTasks {
		if !topics[task.V.TopicID] {
			tasks = append(tasks, task)
		}
	}
	assessment.SeminarTasks = tasks
}

// hoursByPath lists every hours row of the curriculum by its JSON Pointer.
func hoursByPath(root *program.Root) map[string]program.Hours {
	rows := map[string]program.Hours{}
	for mi, m := range root.Curriculum.V.Modules {
		rows[fmt.Sprintf("/curriculum/modules/%d/hours", mi)] = m.V.Hours
		for ti, t := range m.V.Topics {
			rows[fmt.Sprintf("/curriculum/modules/%d/topics/%d/hours", mi, ti)] = t.V.Hours
		}
	}
	rows["/curriculum/totals"] = root.Curriculum.V.Totals
	if root.FinalAttestation != nil {
		rows["/finalAttestation/hours"] = root.FinalAttestation.V.Hours
	}
	return rows
}

// changedPaths lists the rebuilt modules, then every hours row of the other
// modules that the reallocation moved, then the totals.
func changedPaths(root *program.Root, rebuilt []string, before map[string]program.Hours) []string {
	var paths []string
	rebuiltIndex := map[int]bool{}
	for mi, m := range root.Curriculum.V.Modules {
		if slices.Contains(rebuilt, m.V.ID) {
			rebuiltIndex[mi] = true
			paths = append(paths, fmt.Sprintf("/curriculum/modules/%d", mi))
		}
	}

	after := hoursByPath(root)
	var moved []string
	for path, hours := range after {
		if old, existed := before[path]; existed && old == hours {
			continue
		}
		if underRebuilt(path, rebuiltIndex) {
			continue
		}
		moved = append(moved, path)
	}
	sort.Strings(moved)
	return append(paths, moved...)
}

func underRebuilt(path string, rebuilt map[int]bool) bool {
	for mi := range rebuilt {
		prefix := fmt.Sprintf("/curriculum/modules/%d/", mi)
		if len(path) > len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
