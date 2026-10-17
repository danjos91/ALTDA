package content

import "errors"

var (
	// ErrInvalidProgram: the program lacks what content generation needs.
	ErrInvalidProgram = errors.New("invalid program")

	// ErrUnknownNode: a node ID in nodeIds is not in the program.
	ErrUnknownNode = errors.New("unknown node")

	// ErrGeneratorFailed: the generator could not produce content.
	ErrGeneratorFailed = errors.New("content generation failed")

	// ErrGeneratorTimeout: the generator ran out of time.
	ErrGeneratorTimeout = errors.New("content generation timed out")
)
