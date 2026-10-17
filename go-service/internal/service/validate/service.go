package validate

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

// ErrInvalidProgram: the program lacks what validation needs.
var ErrInvalidProgram = errors.New("invalid program")

// statusOrder is the lifecycle of program.status.
var statusOrder = []string{
	"intake", "clarifying", "skeleton_proposed", "skeleton_approved",
	"content_generating", "content_ready", "validated", "exported",
}

// ruleSets run in order; each adds its checks to the collector. A new group
// of rules is a new function here.
var ruleSets = []func(*checker, *program.Root){
	validateHours,
	validateCalendar,
	validateCoverage,
	validateReferences,
	validateContent,
	validateTools,
	validateSources,
	validateSections,
}

type service struct {
	now func() time.Time
}

func NewService() Service {
	return &service{now: time.Now}
}

func (s *service) Validate(_ context.Context, p *program.Program) (*Report, error) {
	root := &p.V

	status := slices.Index(statusOrder, root.Status)
	if status < 0 {
		return nil, fmt.Errorf("%w: unknown status %q", ErrInvalidProgram, root.Status)
	}

	c := &checker{checks: []Check{}}

	// Nothing to verify before a skeleton exists.
	if status >= slices.Index(statusOrder, "skeleton_proposed") {
		switch {
		case root.Header == nil:
			return nil, fmt.Errorf("%w: header is missing", ErrInvalidProgram)
		case root.Curriculum == nil:
			return nil, fmt.Errorf("%w: curriculum is missing", ErrInvalidProgram)
		case root.FinalAttestation == nil:
			return nil, fmt.Errorf("%w: finalAttestation is missing", ErrInvalidProgram)
		}

		for _, ruleSet := range ruleSets {
			ruleSet(c, root)
		}
	}

	return &Report{
		GeneratedAt: s.now().UTC(),
		Summary:     c.summary(),
		Checks:      c.checks,
	}, nil
}
