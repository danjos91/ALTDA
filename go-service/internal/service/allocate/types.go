package allocate

import (
	"context"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

// Service recomputes the hours of a program after manual edits.
type Service interface {
	// Allocate updates p in place: nodes locked by a person (_meta.locked)
	// keep their hours, every other topic is reallocated by weight so that
	// the plan adds up to header.totalHours exactly.
	Allocate(ctx context.Context, p *program.Program) error
}
