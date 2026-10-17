package program

import (
	"encoding/json"
	"fmt"
)

// Object is a JSON object.
type Object map[string]any

// Array is a JSON array.
type Array []any

func AsObject(v any) (Object, bool) {
	switch t := v.(type) {
	case Object:
		return t, true
	case map[string]any:
		return Object(t), true
	default:
		return nil, false
	}
}

func AsArray(v any) (Array, bool) {
	switch t := v.(type) {
	case Array:
		return t, true
	case []any:
		return Array(t), true
	default:
		return nil, false
	}
}

func Clone(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func CloneObject(o Object) Object {
	if o == nil {
		return nil
	}
	c, _ := AsObject(Clone(o))
	return c
}

func GetString(o Object, key, fallback string) string {
	if o == nil {
		return fallback
	}
	v, ok := o[key]
	if !ok || v == nil {
		return fallback
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return fallback
	}
	return s
}

func GetFloat(o Object, key string, fallback float64) float64 {
	if o == nil {
		return fallback
	}
	v, ok := o[key]
	if !ok || v == nil {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return fallback
		}
		return f
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return fallback
	}
}

func GetBool(o Object, key string, fallback bool) bool {
	if o == nil {
		return fallback
	}
	v, ok := o[key]
	if !ok || v == nil {
		return fallback
	}
	b, ok := v.(bool)
	if !ok {
		return fallback
	}
	return b
}

func Nested(o Object, keys ...string) Object {
	cur := o
	for _, k := range keys {
		if cur == nil {
			return nil
		}
		next, ok := AsObject(cur[k])
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

func EnsureObject(o Object, key string) Object {
	if o == nil {
		return nil
	}
	if existing, ok := AsObject(o[key]); ok && existing != nil {
		return existing
	}
	child := Object{}
	o[key] = child
	return child
}

func EnsureArray(o Object, key string) Array {
	if o == nil {
		return nil
	}
	if existing, ok := AsArray(o[key]); ok && existing != nil {
		return existing
	}
	child := Array{}
	o[key] = child
	return child
}

func Pathf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
