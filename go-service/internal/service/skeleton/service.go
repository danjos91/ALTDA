package skeleton

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	allocateservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocate"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

type service struct {
	drafter     Drafter
	regenerator Regenerator
	allocator   allocator.Config
	reallocate  allocateservice.Service
}

func NewService(drafter Drafter, regenerator Regenerator, allocatorConfig allocator.Config) Service {
	return &service{
		drafter:     drafter,
		regenerator: regenerator,
		allocator:   allocatorConfig,
		reallocate:  allocateservice.NewService(allocatorConfig),
	}
}

func (s *service) Generate(ctx context.Context, p *program.Program) ([]Variant, error) {
	return s.generate(ctx, p, "")
}

func (s *service) generate(ctx context.Context, p *program.Program, comment string) ([]Variant, error) {
	req, err := requestFrom(p)
	if err != nil {
		return nil, err
	}
	req.Comment = comment

	drafts, err := s.drafter.Draft(ctx, req)
	if err != nil {
		return nil, err
	}

	var variants []Variant
	for _, draft := range drafts {
		if len(variants) == maxVariants {
			break
		}

		draft.Normalize()
		if failed := validate.Failed(validate.CheckDraft(draft)); len(failed) > 0 {
			slog.WarnContext(ctx, "draft rejected", slog.String("key", draft.Key), slog.String("reason", failed[0].Message))
			continue
		}

		variant, err := s.allocate(ctx, draft, req)
		if errors.Is(err, allocator.ErrNotEnoughHours) {
			slog.WarnContext(ctx, "draft has too many topics for the hours", slog.String("key", draft.Key), slog.Any("error", err))
			continue
		}
		if err != nil {
			return nil, err
		}
		variants = append(variants, *variant)
	}

	if len(variants) == 0 {
		return nil, ErrNoValidVariant
	}
	return variants, nil
}

// requestFrom reads request.params; the hours may also come from a header
// that already exists.
func requestFrom(p *program.Program) (Request, error) {
	root := &p.V
	if root.Request == nil || root.Request.V.Params == nil {
		return Request{}, fmt.Errorf("%w: request.params is missing", ErrInvalidProgram)
	}
	params := root.Request.V.Params.V

	hours := 0.0
	switch {
	case params.TotalHours != nil:
		hours = *params.TotalHours
	case root.Header != nil:
		hours = root.Header.V.TotalHours
	}
	if hours <= 0 {
		return Request{}, fmt.Errorf("%w: request.params.totalHours is required", ErrInvalidProgram)
	}

	req := Request{
		Title:                valueOr(params.Title, "Программа"),
		Domain:               valueOr(params.Domain, ""),
		ProgramType:          valueOr(params.ProgramType, ""),
		Audience:             valueOr(params.Audience, ""),
		EntryLevel:           valueOr(params.EntryLevel, ""),
		StudyForm:            valueOr(params.StudyForm, "очная"),
		FinalAttestationForm: valueOr(params.FinalAttestationForm, ""),
		TotalHours:           hours,
		MinPracticeShare:     params.MinPracticeShare,
	}
	if params.IsInternship != nil {
		req.IsInternship = *params.IsInternship
	}
	return req, nil
}

func valueOr(s *string, fallback string) string {
	if s == nil || *s == "" {
		return fallback
	}
	return *s
}

// allocate gives the draft its hours.
func (s *service) allocate(ctx context.Context, draft skeleton.Draft, req Request) (*Variant, error) {
	in := allocator.Input{
		TotalHours:       req.TotalHours,
		StudyForm:        req.StudyForm,
		IsInternship:     req.IsInternship,
		MinPracticeShare: req.MinPracticeShare,
	}
	for _, m := range draft.Modules {
		module := allocator.Module{ID: m.ID, Weight: m.Weight, IsIntro: m.IsIntro}
		for _, t := range m.Topics {
			topic := allocator.Topic{ID: t.ID, Weight: t.Weight, PracticeShareHint: t.PracticeShareHint}
			for _, kind := range t.Kinds {
				topic.Kinds = append(topic.Kinds, allocator.Kind(kind))
			}
			module.Topics = append(module.Topics, topic)
		}
		in.Modules = append(in.Modules, module)
	}

	result, err := allocator.Allocate(in, s.allocator)
	if err != nil {
		return nil, err
	}
	for _, warning := range result.Warnings {
		slog.WarnContext(ctx, "allocation warning", slog.String("key", draft.Key), slog.String("warning", warning))
	}

	return assemble(draft, result), nil
}

func assemble(draft skeleton.Draft, result *allocator.Result) *Variant {
	variant := &Variant{
		Key:          draft.Key,
		Rationale:    draft.Rationale,
		Pros:         draft.Pros,
		Cons:         draft.Cons,
		Goal:         draft.Goal,
		Competencies: draft.Competencies,
		FinalAttestation: VariantFinalAttestation{
			Form:        draft.FinalAttestation.Form,
			Description: draft.FinalAttestation.Description,
			OutcomeIDs:  draft.FinalAttestation.OutcomeIDs,
			Hours:       result.FinalAttestation,
		},
		Totals: result.Totals,
	}

	for mi, m := range draft.Modules {
		module := VariantModule{
			ID: m.ID, Title: m.Title, IsIntro: m.IsIntro, Weight: m.Weight,
			Hours: result.Modules[mi].Hours,
		}
		for ti, t := range m.Topics {
			module.Topics = append(module.Topics, VariantTopic{
				ID: t.ID, Title: t.Title, Weight: t.Weight, Kinds: t.Kinds,
				PracticeShareHint: t.PracticeShareHint, OutcomeIDs: t.OutcomeIDs,
				Prerequisites: t.Prerequisites, Hours: result.Modules[mi].Topics[ti].Hours,
			})
		}
		variant.Modules = append(variant.Modules, module)
	}
	return variant
}
