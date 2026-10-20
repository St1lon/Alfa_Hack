// Package config загружает конфигурацию сервисов из env — по блоку на концерн
// (CLAUDE.md §7). Все переменные перечислены в .env.example.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
)

// HTTPConfig — сетевые параметры HTTP-сервера сервиса.
type HTTPConfig struct {
	Host string `env:"HTTP_HOST" envDefault:"0.0.0.0"`
	Port int    `env:"HTTP_PORT" envDefault:"8080" validate:"min=1,max=65535"`
	// ReadHeaderTimeout нужен отдельно от ReadTimeout: иначе клиент, шлющий
	// заголовки по байту, держит соединение сколько угодно (slowloris).
	ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"5s"`
	ReadTimeout       time.Duration `env:"HTTP_READ_TIMEOUT"        envDefault:"15s"`
	// WriteTimeout=0 означает «без ограничения» — так нужно сервисам со SSE
	// и долгими вызовами моделей.
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT"    envDefault:"0s"`
	IdleTimeout     time.Duration `env:"HTTP_IDLE_TIMEOUT"     envDefault:"60s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

// PostgresConfig — параметры подключения к общей базе проекта.
type PostgresConfig struct {
	Host     string `env:"PG_HOST"      envDefault:"localhost" validate:"required"`
	Port     int    `env:"PG_PORT"      envDefault:"5432" validate:"min=1,max=65535"`
	User     string `env:"PG_USER"      validate:"required"`
	Password string `env:"PG_PASSWORD"  validate:"required"`
	Database string `env:"PG_DATABASE"  validate:"required"`
	SSLMode  string `env:"PG_SSLMODE"   envDefault:"disable"`
	MaxConns int32  `env:"PG_MAX_CONNS" envDefault:"10" validate:"min=1"`
}

// parse читает env в cfg и валидирует результат.
func parse(cfg any, validate *validator.Validate) error {
	if err := env.Parse(cfg); err != nil {
		return fmt.Errorf("parse env: %w", err)
	}
	if err := validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
}
