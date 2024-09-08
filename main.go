package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/nats-io/nats.go"
	"github.com/sandrolain/rules/api"
	"github.com/sandrolain/rules/engine"
)

type Config struct {
	NatsURL           string `env:"NATS_URL" envDefault:"nats://localhost:4222" validate:"required,url"`
	NatsInputSubject  string `env:"NATS_INPUT_SUBJECT" envDefault:"rules.engine.input" validate:"required"`
	NatsOutputSubject string `env:"NATS_OUTPUT_SUBJECT" envDefault:"rules.engine.output" validate:"required"`
	NatsInputStream   string `env:"NATS_INPUT_STREAM" envDefault:"RULES_INPUT" validate:"required"`
	NatsOutputStream  string `env:"NATS_OUTPUT_STREAM" envDefault:"RULES_OUTPUT" validate:"required"`
	LogLevel          string `env:"LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
}

type PolicyResult struct {
	PolicyID string `json:"policy_id"`
	Result   bool   `json:"result"`
	Error    string `json:"error,omitempty"`
}

type InputAck struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func main() {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		slog.Error("Error parsing configuration", "error", err)
		os.Exit(1)
	}

	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		slog.Error("Error validating configuration", "error", err)
		os.Exit(1)
	}

	// Logger configuration
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
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("Error connecting to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		slog.Error("Error creating JetStream context", "error", err)
		os.Exit(1)
	}

	// Create or get the input stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     cfg.NatsInputStream,
		Subjects: []string{cfg.NatsInputSubject},
	})
	if err != nil {
		slog.Error("Error creating input stream", "error", err)
		os.Exit(1)
	}

	// Create or get the output stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     cfg.NatsOutputStream,
		Subjects: []string{cfg.NatsOutputSubject},
	})
	if err != nil {
		slog.Error("Error creating output stream", "error", err)
		os.Exit(1)
	}

	ruleEngine, err := engine.NewRuleEngine()
	if err != nil {
		slog.Error("Error creating rule engine", "error", err)
		os.Exit(1)
	}

	natsHandler, err := api.NewNatsHandler(nc, ruleEngine)
	if err != nil {
		slog.Error("Error creating NATS handler", "error", err)
		os.Exit(1)
	}

	if err := natsHandler.HandleRequests(); err != nil {
		slog.Error("Error setting up NATS handlers", "error", err)
		os.Exit(1)
	}

	// Subscribe to the input subject using JetStream
	sub, err := js.Subscribe(cfg.NatsInputSubject, func(m *nats.Msg) {
		var input map[string]interface{}
		if err := json.Unmarshal(m.Data, &input); err != nil {
			slog.Error("Error parsing input", "error", err)
			sendInputAck(js, m, false, "Error parsing input")
			return
		}

		policies := ruleEngine.GetAllPolicies()
		results := make([]PolicyResult, 0, len(policies))

		for _, policy := range policies {
			shouldExecute, err := policy.ShouldExecute(input)
			if err != nil {
				slog.Error("Error evaluating CEL expression", "error", err, "policy_id", policy.ID)
				results = append(results, PolicyResult{PolicyID: policy.ID, Result: false, Error: err.Error()})
				break
			}

			if !shouldExecute {
				slog.Debug("Policy will not be executed for this input", "policy_id", policy.ID)
				continue
			}

			result, err := ruleEngine.EvaluatePolicy(policy.ID, input)
			if err != nil {
				slog.Error("Error evaluating policy", "error", err, "policy_id", policy.ID)
				results = append(results, PolicyResult{PolicyID: policy.ID, Result: false, Error: err.Error()})
				break
			}

			results = append(results, PolicyResult{PolicyID: policy.ID, Result: result})
			if !result {
				break
			}

			slog.Info("Policy result", "policy_id", policy.ID, "result", result)
		}

		// Send results to output subject using JetStream
		resultsJSON, err := json.Marshal(results)
		if err != nil {
			slog.Error("Error marshaling results", "error", err)
			sendInputAck(js, m, false, "Error marshaling results")
			return
		}

		_, err = js.Publish(cfg.NatsOutputSubject, resultsJSON)
		if err != nil {
			slog.Error("Error publishing results", "error", err)
			sendInputAck(js, m, false, "Error publishing results")
			return
		}

		// Send acknowledgment
		sendInputAck(js, m, true, "Input processed successfully")
	})

	if err != nil {
		slog.Error("Error subscribing to NATS topic", "error", err)
		os.Exit(1)
	}

	// Set up signal handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	slog.Info("Waiting for input on NATS...", "input_subject", cfg.NatsInputSubject, "output_subject", cfg.NatsOutputSubject)

	// Wait for termination signal
	<-stop

	slog.Info("Shutting down gracefully...")

	if err := sub.Unsubscribe(); err != nil {
		slog.Error("Error unsubscribing from NATS", "error", err)
	}

	if err := nc.Drain(); err != nil {
		slog.Error("Error draining NATS connection", "error", err)
	}

	// Attendi la chiusura della connessione
	nc.Close()
	slog.Info("NATS connection closed")

	slog.Info("Shutdown complete")
}

func sendInputAck(js nats.JetStreamContext, m *nats.Msg, success bool, message string) {
	ack := InputAck{
		Success: success,
		Message: message,
	}
	ackJSON, err := json.Marshal(ack)
	if err != nil {
		slog.Error("Error marshaling acknowledgment", "error", err)
		return
	}
	if err := m.Ack(); err != nil {
		slog.Error("Error acknowledging message", "error", err)
	}
	if _, err := js.Publish(m.Reply, ackJSON); err != nil {
		slog.Error("Error sending acknowledgment", "error", err)
	}
}
