package app

import "github.com/VladimirAzanza/ALTDA/go-service/internal/env"

type Config struct {
	Env      string
	LogLevel string
}

func NewConfig() *Config {
	return &Config{
		Env:      env.String("ENV", "local"),
		LogLevel: env.String("LOG_LEVEL", "debug"),
	}
}
