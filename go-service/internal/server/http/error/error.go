package httperror

import "errors"

type Code string

const (
	BadRequest    Code = "bad_request"
	NodeLocked    Code = "node_locked"
	LLMTimeout    Code = "llm_timeout"
	LLMError      Code = "llm_error"
	SchemaInvalid Code = "schema_invalid"

	// Not part of the contract.
	NotImplemented Code = "not_implemented"
	Internal       Code = "internal"
)

type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.Err.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap attaches an underlying cause to a coded error.
func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

// From returns the *Error inside err, or an Internal one if there is none.
func From(err error) *Error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return &Error{Code: Internal, Message: "internal error", Err: err}
}
