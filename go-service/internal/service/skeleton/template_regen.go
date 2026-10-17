package skeleton

import (
	"context"
	"fmt"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

type templateRegenerator struct{}

// NewTemplateRegenerator returns the stand-in regenerator. It cannot read a
// comment: it rebuilds each requested module as the template would have made
// it for the current request, which restores a module someone has changed.
func NewTemplateRegenerator() Regenerator {
	return templateRegenerator{}
}

func (templateRegenerator) Regenerate(_ context.Context, in RegenerateInput) ([]skeleton.Module, error) {
	if len(in.Outcomes) == 0 {
		return nil, fmt.Errorf("the program has no outcomes to teach")
	}

	s := shapeOf(in.Current)
	total := 0
	for _, m := range in.Current {
		total += len(m.Topics)
	}

	kinds := kindsFor(s, in.Request)
	topicHint := s.practiceHint
	if topicHint != nil && in.Request.MinPracticeShare != nil && *in.Request.MinPracticeShare > *topicHint {
		topicHint = in.Request.MinPracticeShare
	}

	var rebuilt []skeleton.Module
	offset := 0
	for k, current := range in.Current {
		count := len(current.Topics)
		if contains(in.Targets, current.ID) {
			rebuilt = append(rebuilt, rebuildModule(s, current, k, offset, count, total, in.Outcomes, kinds, topicHint))
		}
		offset += count
	}
	return rebuilt, nil
}

// shapeOf finds the template shape a program was built from, by the titles of
// its modules; programs that do not match any use the first one.
func shapeOf(modules []skeleton.Module) shape {
	for _, s := range shapes {
		for _, m := range modules {
			for _, stage := range s.stages {
				if m.Title == stage {
					return s
				}
			}
		}
	}
	return shapes[0]
}

func rebuildModule(s shape, current skeleton.Module, index, offset, count, total int, outcomes, kinds []string, hint *float64) skeleton.Module {
	title := current.Title
	if index < len(s.stages) {
		title = s.stages[index]
	}

	module := skeleton.Module{
		ID:      current.ID,
		Title:   title,
		IsIntro: s.introFirst && index == 0,
		Weight:  current.Weight, // the module keeps its share of the hours
	}

	moduleNo := index + 1
	for j := 0; j < count; j++ {
		topic := skeleton.Topic{
			ID:                fmt.Sprintf("T%d.%d", moduleNo, j+1),
			Title:             topicTitle(title, j, count),
			Weight:            roundTo(1/float64(count), 4),
			Kinds:             kinds,
			PracticeShareHint: hint,
			OutcomeIDs:        []string{outcomes[(offset+j)*len(outcomes)/total]},
			Prerequisites:     []string{},
		}
		if j > 0 {
			topic.Prerequisites = []string{fmt.Sprintf("T%d.%d", moduleNo, j)}
		}
		module.Topics = append(module.Topics, topic)
	}

	// Rounding must not leave the topic weights short of 1.
	var sum float64
	for _, t := range module.Topics {
		sum += t.Weight
	}
	last := &module.Topics[len(module.Topics)-1]
	last.Weight = roundTo(last.Weight+(1-sum), 4)
	return module
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
