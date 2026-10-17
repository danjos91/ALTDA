package httperror

import (
	"errors"
	"fmt"
	"testing"
)

func TestFromFindsWrappedError(t *testing.T) {
	err := fmt.Errorf("service: %w", New(NodeLocked, "node M2 is locked"))

	got := From(err)
	if got.Code != NodeLocked || got.Message != "node M2 is locked" {
		t.Errorf("got %+v", got)
	}
}

func TestFromPlainErrorIsInternal(t *testing.T) {
	cause := errors.New("boom")

	got := From(cause)
	if got.Code != Internal {
		t.Errorf("code = %q, want %q", got.Code, Internal)
	}
	if !errors.Is(got, cause) {
		t.Error("cause must stay reachable with errors.Is")
	}
}
