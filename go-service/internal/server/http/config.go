package http

import (
	"net"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/env"
)

type Config struct {
	Host string
	Port string
}

func NewConfig() *Config {
	return &Config{
		Host: env.String("HOST", "0.0.0.0"),
		Port: env.String("HTTP_PORT", "8081"),
	}
}

func (c *Config) Address() string {
	return net.JoinHostPort(c.Host, c.Port)
}
