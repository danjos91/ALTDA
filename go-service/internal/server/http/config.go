package http

import (
	"net"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/env"
)

type Config struct {
	Host string
	Port string

	// WriteTimeout must exceed the longest handler deadline (content ~870s → default 16m).
	WriteTimeout time.Duration

	SkeletonDeadline time.Duration
	ContentDeadline  time.Duration
}

func NewConfig() *Config {
	writeMin := env.Int("ALTDA_HTTP_WRITE_TIMEOUT_MIN", 16)
	if writeMin < 1 {
		writeMin = 16
	}
	skelSec := env.Int("ALTDA_SKELETON_DEADLINE_SEC", 280)
	if skelSec < 1 {
		skelSec = 280
	}
	contentSec := env.Int("ALTDA_CONTENT_DEADLINE_SEC", 870)
	if contentSec < 1 {
		contentSec = 870
	}
	return &Config{
		Host:             env.String("HOST", "0.0.0.0"),
		Port:             env.String("HTTP_PORT", "8081"),
		WriteTimeout:     time.Duration(writeMin) * time.Minute,
		SkeletonDeadline: time.Duration(skelSec) * time.Second,
		ContentDeadline:  time.Duration(contentSec) * time.Second,
	}
}

func (c *Config) Address() string {
	return net.JoinHostPort(c.Host, c.Port)
}
