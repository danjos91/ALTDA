package allocate

import (
	"errors"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
)

var (
	// ErrInvalidProgram: the program lacks what the allocation needs
	// (header, curriculum, final attestation, topics...).
	ErrInvalidProgram = errors.New("invalid program")

	ErrNotEnoughHours   = allocator.ErrNotEnoughHours
	ErrNothingToBalance = allocator.ErrNothingToBalance
)
