// Package dto holds response shapes shared by several handlers.
package dto

import "github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"

// Issue is a failed check as the contract lists it in "issues".
type Issue struct {
	RuleID   string            `json:"ruleId"`
	Severity validate.Severity `json:"severity"`
	Message  string            `json:"message"`
	Path     string            `json:"path"`
}

// IssuesFrom converts failed checks to contract issues. The result is never
// nil, so it encodes as [] and not null.
func IssuesFrom(checks []validate.Check) []Issue {
	issues := make([]Issue, 0, len(checks))
	for _, c := range checks {
		if c.Passed {
			continue
		}
		issues = append(issues, Issue{RuleID: c.RuleID, Severity: c.Severity, Message: c.Message, Path: c.Path})
	}
	return issues
}
