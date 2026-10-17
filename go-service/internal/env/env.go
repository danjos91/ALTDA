// Package env reads configuration from environment variables with defaults.
package env

import (
	"os"
	"strconv"
	"strings"
)

func String(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return defaultValue
}

func Int(key string, defaultValue int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return defaultValue
	}
	return n
}

func Bool(key string, defaultValue bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return defaultValue
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}
