package content

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
)

type service struct {
	generator Generator
	validator validate.Service
	config    Config
}

func NewService(generator Generator, validator validate.Service, config Config) Service {
	if config.MaxParallel < 1 {
		config.MaxParallel = 1
	}
	return &service{generator: generator, validator: validator, config: config}
}

func (s *service) Generate(ctx context.Context, p *program.Program, nodeIDs []string) ([]validate.Check, error) {
	root := &p.V
	if err := requireStructure(root); err != nil {
		return nil, err
	}

	targets, err := resolveTargets(root, nodeIDs)
	if err != nil {
		return nil, err
	}

	refs := topicRefs(root)
	skipLocked(refs, targets)

	generated, err := s.generateTopics(ctx, root, refs, targets)
	if err != nil {
		return nil, err
	}

	merge(root, refs, generated, targets)

	sessionHours, sessionsPerWeek := s.calendarParams(root)
	root.Calendar = &program.Node[program.Calendar]{V: program.Calendar{
		Sessions: buildCalendar(root, sessionHours, sessionsPerWeek),
	}}
	numbers := sessionsByTopic(root.Calendar.V.Sessions)
	for i := range root.Lectures {
		root.Lectures[i].V.SessionNumbers = numbers[root.Lectures[i].V.TopicID]
	}

	updateSections(root, s.generator.Placeholder())
	root.Status = "content_ready"

	report, err := s.validator.Validate(ctx, p)
	if err != nil {
		return nil, err
	}
	return report.Issues(), nil
}

func requireStructure(root *program.Root) error {
	switch {
	case root.Header == nil:
		return fmt.Errorf("%w: header is missing", ErrInvalidProgram)
	case root.Curriculum == nil || len(root.Curriculum.V.Modules) == 0:
		return fmt.Errorf("%w: curriculum has no modules", ErrInvalidProgram)
	case root.FinalAttestation == nil:
		return fmt.Errorf("%w: finalAttestation is missing", ErrInvalidProgram)
	}
	for _, m := range root.Curriculum.V.Modules {
		if len(m.V.Topics) == 0 {
			return fmt.Errorf("%w: module %s has no topics", ErrInvalidProgram, m.V.ID)
		}
	}
	return nil
}

// skipLocked drops the topics a person locked, or whose module is locked.
func skipLocked(refs []topicRef, targets target) {
	for _, ref := range refs {
		if ref.topic.IsLocked() || ref.module.IsLocked() {
			delete(targets, ref.topic.ID)
		}
	}
}

func (s *service) calendarParams(root *program.Root) (sessionHours, sessionsPerWeek float64) {
	sessionHours, sessionsPerWeek = s.config.SessionHours, s.config.SessionsPerWeek
	if root.Request != nil && root.Request.V.Params != nil {
		params := root.Request.V.Params.V
		if params.SessionHours != nil && *params.SessionHours > 0 {
			sessionHours = *params.SessionHours
		}
		if params.SessionsPerWeek != nil && *params.SessionsPerWeek > 0 {
			sessionsPerWeek = *params.SessionsPerWeek
		}
	}
	return sessionHours, sessionsPerWeek
}

// generateTopics runs the generator for every targeted topic, at most
// MaxParallel at a time. Results are kept by topic, so the order in which the
// goroutines finish never shows in the program.
func (s *service) generateTopics(ctx context.Context, root *program.Root, refs []topicRef, targets target) (map[string]TopicContent, error) {
	var inputs []TopicInput
	for _, ref := range refs {
		if targets[ref.topic.ID] != 0 {
			inputs = append(inputs, inputFor(root, ref))
		}
	}

	results := make([]TopicContent, len(inputs))
	errs := make([]error, len(inputs))

	var wg sync.WaitGroup
	slots := make(chan struct{}, s.config.MaxParallel)
	for i := range inputs {
		if ctx.Err() != nil {
			errs[i] = ctx.Err()
			continue
		}
		slots <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-slots; wg.Done() }()
			results[i], errs[i] = s.generator.Topic(ctx, inputs[i])
		}()
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrGeneratorTimeout, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrGeneratorFailed, err)
	}

	byTopic := make(map[string]TopicContent, len(inputs))
	for i, in := range inputs {
		byTopic[in.Topic.ID] = results[i]
	}
	return byTopic, nil
}

func inputFor(root *program.Root, ref topicRef) TopicInput {
	text := map[string]string{}
	var all []OutcomeInfo
	for _, c := range root.Competencies {
		for _, group := range [][]program.Node[program.Outcome]{c.V.Knowledge, c.V.Skills, c.V.Abilities} {
			for _, o := range group {
				text[o.V.ID] = o.V.Text
				all = append(all, OutcomeInfo{ID: o.V.ID, Text: o.V.Text})
			}
		}
	}

	var own []OutcomeInfo
	for _, id := range ref.topic.OutcomeIDs {
		own = append(own, OutcomeInfo{ID: id, Text: text[id]})
	}

	title := root.Header.V.Title
	domain := root.Header.V.ItDirection
	if root.Request != nil && root.Request.V.Params != nil {
		params := root.Request.V.Params.V
		if title == "" && params.Title != nil {
			title = *params.Title
		}
		if domain == "" && params.Domain != nil {
			domain = *params.Domain
		}
	}

	return TopicInput{
		ProgramTitle: title,
		Domain:       domain,
		Module:       ModuleInfo{ID: ref.module.ID, Title: ref.module.Title},
		Topic:        TopicInfo{ID: ref.topic.ID, Title: ref.topic.Title, Hours: ref.topic.Hours},
		Outcomes:     own,
		AllOutcomes:  all,
	}
}
