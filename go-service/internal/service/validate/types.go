package validate

import (
	"context"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

// Service checks a program against the validation rules of docs/API.md.
// Rule IDs, categories and severities match tools/validate_reference.py.
type Service interface {
	Validate(ctx context.Context, p *program.Program) (*Report, error)
}

type Category string

const (
	CategoryHours        Category = "hours"
	CategoryCoverage     Category = "coverage"
	CategorySources      Category = "sources"
	CategoryCompleteness Category = "completeness"
	CategoryConsistency  Category = "consistency"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Report is $defs/validationReport of the schema.
type Report struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Summary     Summary   `json:"summary"`
	Checks      []Check   `json:"checks"`
}

type Summary struct {
	Errors            int `json:"errors"`
	Warnings          int `json:"warnings"`
	Passed            int `json:"passed"`
	UnverifiedSources int `json:"unverifiedSources"`
}

// Check is one rule evaluated on one node. Passed checks are kept: the report
// shows what was verified, not only what failed.
type Check struct {
	RuleID     string   `json:"ruleId"`
	Category   Category `json:"category"`
	Severity   Severity `json:"severity"`
	Passed     bool     `json:"passed"`
	Message    string   `json:"message"`
	Path       string   `json:"path,omitempty"`
	RelatedIDs []string `json:"relatedIds,omitempty"`
}

// Issues are the checks that did not pass.
func (r *Report) Issues() []Check {
	issues := []Check{}
	for _, c := range r.Checks {
		if !c.Passed {
			issues = append(issues, c)
		}
	}
	return issues
}
