// Package env reads configuration from environment variables with defaults.
package env

import "os"

func String(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return defaultValue
}
