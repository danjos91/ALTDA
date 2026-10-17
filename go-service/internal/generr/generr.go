// Package generr holds typed generation errors shared by the LLM client and
// the skeleton/content services without creating import cycles.
package generr

import "errors"

var (
	ErrTimeout       = errors.New("llm_timeout")
	ErrUpstream      = errors.New("llm_error")
	ErrSchemaInvalid = errors.New("schema_invalid")
)

// IsContract reports whether err is (or wraps) a typed LLM contract error.
func IsContract(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrUpstream) || errors.Is(err, ErrSchemaInvalid)
}
