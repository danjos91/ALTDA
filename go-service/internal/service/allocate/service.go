package allocate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
)

type service struct {
	config allocator.Config
}

func NewService(config allocator.Config) Service {
	return &service{config: config}
}

func (s *service) Allocate(ctx context.Context, p *program.Program) error {
	root := &p.V

	switch {
	case root.Header == nil:
		return fmt.Errorf("%w: header is missing", ErrInvalidProgram)
	case root.Curriculum == nil || len(root.Curriculum.V.Modules) == 0:
		return fmt.Errorf("%w: curriculum has no modules", ErrInvalidProgram)
	case root.FinalAttestation == nil:
		return fmt.Errorf("%w: finalAttestation is missing", ErrInvalidProgram)
	case root.Header.V.TotalHours <= 0:
		return fmt.Errorf("%w: header.totalHours must be positive", ErrInvalidProgram)
	}

	in := buildInput(root)

	result, err := allocator.Allocate(in, s.config)
	if errors.Is(err, allocator.ErrInvalidInput) {
		return fmt.Errorf("%w: %v", ErrInvalidProgram, err)
	}
	if err != nil {
		return err
	}

	for _, warning := range result.Warnings {
		slog.WarnContext(ctx, "allocation warning", slog.String("warning", warning))
	}

	apply(root, result)
	return nil
}

func apply(root *program.Root, result *allocator.Result) {
	modules := root.Curriculum.V.Modules
	for mi := range modules {
		module := &modules[mi].V
		module.Hours = fromAllocator(result.Modules[mi].Hours)
		for ti := range module.Topics {
			module.Topics[ti].V.Hours = fromAllocator(result.Modules[mi].Topics[ti].Hours)
		}
	}
	root.Curriculum.V.Totals = fromAllocator(result.Totals)
	root.FinalAttestation.V.Hours = fromAllocator(result.FinalAttestation)
}
