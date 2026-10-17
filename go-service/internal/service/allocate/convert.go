package allocate

import (
	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
)

// buildInput reads the program into what the allocator needs. The program
// stores no topic "kinds" (only the skeleton draft does), so they are read
// from which hour columns are in use today; that also keeps each topic's
// current practice/lecture balance.
func buildInput(root *program.Root) allocator.Input {
	header := root.Header.V
	in := allocator.Input{
		TotalHours: header.TotalHours,
		StudyForm:  header.StudyForm,
	}

	if root.Request != nil && root.Request.V.Params != nil {
		params := root.Request.V.Params.V
		in.MinPracticeShare = params.MinPracticeShare
		in.IsInternship = params.IsInternship != nil && *params.IsInternship
	}

	if root.FinalAttestation.V.IsLocked() {
		hours := root.FinalAttestation.V.Hours.Total
		in.FixedAttestation = &hours
	}

	for _, moduleNode := range root.Curriculum.V.Modules {
		module := moduleNode.V
		out := allocator.Module{
			ID:      module.ID,
			Weight:  weightOr(module.Weight, module.Hours.Total),
			IsIntro: module.IsIntro != nil && *module.IsIntro,
		}

		for _, topicNode := range module.Topics {
			topic := topicNode.V
			out.Topics = append(out.Topics, allocator.Topic{
				ID:                topic.ID,
				Weight:            weightOr(topic.Weight, topic.Hours.Total),
				Kinds:             kindsOf(topic.Hours),
				PracticeShareHint: practiceShareOf(topic.Hours),
				Fixed:             fixedOf(module, topic),
			})
		}
		in.Modules = append(in.Modules, out)
	}
	return in
}

// fixedOf returns the topic's hours when it, or its module, is locked.
func fixedOf(module program.Module, topic program.Topic) *allocator.Hours {
	if !module.IsLocked() && !topic.IsLocked() {
		return nil
	}
	hours := toAllocator(topic.Hours)
	return &hours
}

// weightOr prefers the weight the LLM proposed, then the hours the node has
// now, then an equal share.
func weightOr(weight *float64, currentHours float64) float64 {
	switch {
	case weight != nil && *weight > 0:
		return *weight
	case currentHours > 0:
		return currentHours
	default:
		return 1
	}
}

func kindsOf(h program.Hours) []allocator.Kind {
	var kinds []allocator.Kind
	if h.Lecture > 0 {
		kinds = append(kinds, allocator.KindLecture)
	}
	if h.Practice > 0 {
		kinds = append(kinds, allocator.KindPractice)
	}
	if h.Lab > 0 {
		kinds = append(kinds, allocator.KindLab)
	}
	if h.SelfStudy > 0 {
		kinds = append(kinds, allocator.KindSelfStudy)
	}
	if len(kinds) == 0 {
		// Nothing allocated yet: assume the usual mix.
		kinds = []allocator.Kind{allocator.KindLecture, allocator.KindPractice}
	}
	return kinds
}

// practiceShareOf keeps the topic's current balance between lecture and
// practice/lab; nil when it has only one side.
func practiceShareOf(h program.Hours) *float64 {
	practical := h.Practice + h.Lab
	if h.Lecture <= 0 || practical <= 0 {
		return nil
	}
	share := practical / (h.Lecture + practical)
	return &share
}

func toAllocator(h program.Hours) allocator.Hours {
	return allocator.Hours(h)
}

func fromAllocator(h allocator.Hours) program.Hours {
	return program.Hours(h)
}
