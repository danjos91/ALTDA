// Package program is the Go view of program.json (schema/program.schema.json).
//
// The schema is large and keeps growing, and other services own most of it.
// Each node therefore keeps a typed view of the few fields Go reads or writes
// (V) and carries every other field through untouched, so decoding and
// re-encoding a program never loses or invents data.
package program

import (
	"bytes"
	"encoding/json"
	"maps"
)

// Node wraps one JSON object: V is the typed view, raw holds all of its
// original fields, and base is what the typed view encoded to right after
// decoding.
//
// Encoding writes only what changed since decoding: a typed field whose
// encoding is the same as at decode time keeps its original bytes (or stays
// absent), a field that changed is written, and a field the view no longer
// encodes is removed. A node built from scratch has no history, so all of its
// typed fields are written.
type Node[T any] struct {
	V    T
	raw  map[string]json.RawMessage
	base map[string]json.RawMessage
}

func (n *Node[T]) UnmarshalJSON(data []byte) error {
	n.raw, n.base = nil, nil
	if err := json.Unmarshal(data, &n.raw); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &n.V); err != nil {
		return err
	}

	base, err := typedFields(n.V)
	if err != nil {
		return err
	}
	n.base = base
	return nil
}

func (n Node[T]) MarshalJSON() ([]byte, error) {
	now, err := typedFields(n.V)
	if err != nil {
		return nil, err
	}

	out := maps.Clone(n.raw)
	if out == nil {
		out = make(map[string]json.RawMessage, len(now))
	}

	for key, value := range now {
		if before, had := n.base[key]; had && bytes.Equal(before, value) {
			continue // unchanged since decoding: keep what the program had
		}
		out[key] = value
	}
	for key := range n.base {
		if _, still := now[key]; !still {
			delete(out, key) // the view dropped this field
		}
	}
	return json.Marshal(out)
}

// typedFields encodes the typed view as a map of its JSON fields.
func typedFields(view any) (map[string]json.RawMessage, error) {
	data, err := json.Marshal(view)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}
