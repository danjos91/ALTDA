package skeleton

import "errors"

var (
	// ErrInvalidProgram: the request lacks what the skeleton needs (e.g. totalHours).
	ErrInvalidProgram = errors.New("invalid program")

	// ErrNoValidVariant: the drafter returned nothing usable.
	ErrNoValidVariant = errors.New("no valid skeleton variant could be produced")

	// ErrNodeLocked: a node explicitly asked for is locked and locks are respected.
	ErrNodeLocked = errors.New("node is locked")

	// ErrUnknownNode: a node ID in nodeIds is not in the program.
	ErrUnknownNode = errors.New("unknown node")
)
