package content

import (
	"context"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
)

// Service fills in the content of an approved skeleton: activities, lecture
// plans, tests, seminar tasks, the calendar and the template sections.
type Service interface {
	// Generate updates p in place and returns the issues the validator finds in
	// the result. nodeIDs nil means the whole program; otherwise only those
	// nodes and what depends on them are regenerated. Locked nodes are skipped.
	Generate(ctx context.Context, p *program.Program, nodeIDs []string) ([]validate.Check, error)
}

// Generator writes the content of one topic. It is the part an LLM replaces;
// the template generator in template.go is a stand-in that invents nothing
// about the subject. A generator never assigns IDs: the service does.
type Generator interface {
	Topic(ctx context.Context, in TopicInput) (TopicContent, error)

	// Placeholder reports whether the content is template text to be replaced,
	// which the template sections of the program say so.
	Placeholder() bool
}

type TopicInput struct {
	ProgramTitle string
	Domain       string
	Module       ModuleInfo
	Topic        TopicInfo
	Outcomes     []OutcomeInfo // the topic's own outcomes
	AllOutcomes  []OutcomeInfo // every outcome of the program, in order
}

type ModuleInfo struct {
	ID    string
	Title string
}

type TopicInfo struct {
	ID    string
	Title string
	Hours program.Hours
}

type OutcomeInfo struct {
	ID   string
	Text string
}

// TopicContent is what a generator returns for one topic. Activities carry the
// topic's hours; Lecture is nil when the topic has no lecture hours; Task is
// nil when it has no practice or lab hours.
type TopicContent struct {
	Activities []program.Activity
	Lecture    *program.Lecture
	Questions  []program.Question
	Task       *program.SeminarTask
}

// Config tunes the service.
type Config struct {
	// MaxParallel limits how many topics are generated at once.
	MaxParallel int

	// Defaults for the calendar when the request does not say.
	SessionHours    float64
	SessionsPerWeek float64
}

func DefaultConfig() Config {
	return Config{MaxParallel: 8, SessionHours: 4, SessionsPerWeek: 2}
}
