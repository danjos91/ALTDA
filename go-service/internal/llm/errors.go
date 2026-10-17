package llm

import "github.com/VladimirAzanza/ALTDA/go-service/internal/generr"

// Sentinel errors mapped to the HTTP contract by handlers.
var (
	ErrTimeout       = generr.ErrTimeout
	ErrUpstream      = generr.ErrUpstream
	ErrSchemaInvalid = generr.ErrSchemaInvalid
)

// IsContract reports whether err is (or wraps) a typed LLM contract error.
func IsContract(err error) bool {
	return generr.IsContract(err)
}
