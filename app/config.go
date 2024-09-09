package app

import (
	"log/slog"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	NatsURL           string `env:"NATS_URL" envDefault:"nats://localhost:4222" validate:"required,url"`
	NatsInputSubject  string `env:"NATS_INPUT_SUBJECT" envDefault:"rules.engine.input" validate:"required"`
	NatsOutputSubject string `env:"NATS_OUTPUT_SUBJECT" envDefault:"rules.engine.output" validate:"required"`
	NatsInputStream   string `env:"NATS_INPUT_STREAM" envDefault:"RULES_INPUT" validate:"required"`
	NatsOutputStream  string `env:"NATS_OUTPUT_STREAM" envDefault:"RULES_OUTPUT" validate:"required"`
	LogLevel          string `env:"LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func SetupLogger(cfg *Config) *slog.Logger {
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
}
