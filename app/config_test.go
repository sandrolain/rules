package app

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expected    *Config
		expectError bool
	}{
		{
			name:    "Default configuration",
			envVars: map[string]string{},
			expected: &Config{
				NatsURL:           "nats://localhost:4222",
				NatsInputSubject:  "rules.engine.input",
				NatsOutputSubject: "rules.engine.output",
				NatsInputStream:   "RULES_INPUT",
				NatsOutputStream:  "RULES_OUTPUT",
				LogLevel:          "info",
			},
			expectError: false,
		},
		{
			name: "Custom configuration",
			envVars: map[string]string{
				"NATS_URL":            "nats://custom:4222",
				"NATS_INPUT_SUBJECT":  "custom.input",
				"NATS_OUTPUT_SUBJECT": "custom.output",
				"NATS_INPUT_STREAM":   "CUSTOM_INPUT",
				"NATS_OUTPUT_STREAM":  "CUSTOM_OUTPUT",
				"LOG_LEVEL":           "debug",
			},
			expected: &Config{
				NatsURL:           "nats://custom:4222",
				NatsInputSubject:  "custom.input",
				NatsOutputSubject: "custom.output",
				NatsInputStream:   "CUSTOM_INPUT",
				NatsOutputStream:  "CUSTOM_OUTPUT",
				LogLevel:          "debug",
			},
			expectError: false,
		},
		{
			name: "Invalid NATS URL",
			envVars: map[string]string{
				"NATS_URL": "invalid-url",
			},
			expectError: true,
		},
		{
			name: "Invalid log level",
			envVars: map[string]string{
				"LOG_LEVEL": "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				// Unset environment variables
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			}()

			// Load configuration
			cfg, err := LoadConfig()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name          string
		logLevel      string
		expectedLevel slog.Level
	}{
		{"Debug level", "debug", slog.LevelDebug},
		{"Info level", "info", slog.LevelInfo},
		{"Warn level", "warn", slog.LevelWarn},
		{"Error level", "error", slog.LevelError},
		{"Default to Info", "invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{LogLevel: tt.logLevel}
			logger := SetupLogger(cfg)

			// Check if the logger is not nil
			assert.NotNil(t, logger)

			// Check if the log level is set correctly
			handler := logger.Handler().(*slog.TextHandler)
			assert.True(t, handler.Enabled(context.Background(), tt.expectedLevel))
			assert.False(t, handler.Enabled(context.Background(), tt.expectedLevel-1))
		})
	}
}
