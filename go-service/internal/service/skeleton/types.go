package skeleton

import (
	"context"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

// maxVariants is the most the contract allows in one response.
const maxVariants = 3

// Service proposes skeleton variants for a request: structure from a
// drafter, hours from the allocator.
type Service interface {
	Generate(ctx context.Context, p *program.Program) ([]Variant, error)

	// Regenerate redoes part of the skeleton. Before the plan exists (the
	// variants are not merged yet) it returns fresh variants; afterwards it
	// rebuilds the requested modules inside the program. Exactly one of
	// Program and Variants is set in the result.
	Regenerate(ctx context.Context, p *program.Program, in RegenerateParams) (*RegenerateResult, error)
}

type RegenerateParams struct {
	Comment string
	// NodeIDs names modules or topics; empty means "infer them from the
	// comment, or take every module".
	NodeIDs []string
	// RespectLocks refuses to touch locked nodes (explicitly requested ones
	// give ErrNodeLocked; ones merely inferred are skipped).
	RespectLocks bool
}

type RegenerateResult struct {
	Program      *program.Program // set when the plan exists
	Variants     []Variant        // set before the plan exists
	ChangedPaths []string         // JSON Pointers to what changed
}

// Request is what a drafter needs to know, read from request.params.
type Request struct {
	Title                string
	Domain               string
	ProgramType          string
	Audience             string
	EntryLevel           string
	StudyForm            string
	FinalAttestationForm string
	Comment              string // the methodologist's note when something is regenerated
	TotalHours           float64
	IsInternship         bool
	MinPracticeShare     *float64
}

// Drafter proposes the structure of the program: modules, topics, outcomes
// and weights. It never decides hours. The template drafter below is a
// stand-in; an LLM-backed drafter implements the same interface.
type Drafter interface {
	Draft(ctx context.Context, req Request) ([]skeleton.Draft, error)
}

// Variant is a draft with its hours allocated, as the contract returns it.
type Variant struct {
	Key              string                  `json:"key"`
	Rationale        string                  `json:"rationale"`
	Pros             []string                `json:"pros"`
	Cons             []string                `json:"cons"`
	Goal             string                  `json:"goal"`
	Competencies     []skeleton.Competency   `json:"competencies"`
	Modules          []VariantModule         `json:"modules"`
	FinalAttestation VariantFinalAttestation `json:"finalAttestation"`
	Totals           allocator.Hours         `json:"totals"`
}

type VariantModule struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	IsIntro bool            `json:"isIntro"`
	Weight  float64         `json:"weight"`
	Hours   allocator.Hours `json:"hours"`
	Topics  []VariantTopic  `json:"topics"`
}

type VariantTopic struct {
	ID                string          `json:"id"`
	Title             string          `json:"title"`
	Weight            float64         `json:"weight"`
	Kinds             []string        `json:"kinds"`
	PracticeShareHint *float64        `json:"practiceShareHint,omitempty"`
	OutcomeIDs        []string        `json:"outcomeIds"`
	Prerequisites     []string        `json:"prerequisites"`
	Hours             allocator.Hours `json:"hours"`
}

type VariantFinalAttestation struct {
	Form        string          `json:"form"`
	Description string          `json:"description"`
	OutcomeIDs  []string        `json:"outcomeIds"`
	Hours       allocator.Hours `json:"hours"`
}

// Regenerator proposes replacements for some modules of a program whose
// competencies are already fixed. Like the Drafter, it never decides hours.
type Regenerator interface {
	Regenerate(ctx context.Context, in RegenerateInput) ([]skeleton.Module, error)
}

type RegenerateInput struct {
	Request  Request
	Comment  string
	Outcomes []string          // outcome IDs of the program's competencies
	Current  []skeleton.Module // the whole current structure, for context
	Targets  []string          // IDs of the modules to rebuild
}
