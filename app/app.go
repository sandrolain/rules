package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/sandrolain/rules/api"
	"github.com/sandrolain/rules/engine"
)

type App struct {
	cfg        *Config
	logger     *slog.Logger
	nc         *nats.Conn
	js         nats.JetStreamContext
	ruleEngine *engine.RuleEngine
}

func NewApp(cfg *Config) (*App, error) {
	logger := SetupLogger(cfg)
	slog.SetDefault(logger)

	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		return nil, fmt.Errorf("error connecting to NATS: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("error creating JetStream context: %w", err)
	}

	ruleEngine, err := engine.NewRuleEngine()
	if err != nil {
		return nil, fmt.Errorf("error creating rule engine: %w", err)
	}

	return &App{
		cfg:        cfg,
		logger:     logger,
		nc:         nc,
		js:         js,
		ruleEngine: ruleEngine,
	}, nil
}

func (a *App) Run() error {
	if err := a.setupStreams(); err != nil {
		return err
	}

	natsHandler, err := api.NewNatsHandler(a.nc, a.ruleEngine)
	if err != nil {
		return fmt.Errorf("error creating NATS handler: %w", err)
	}

	if err := natsHandler.HandleRequests(); err != nil {
		return fmt.Errorf("error setting up NATS handlers: %w", err)
	}

	sub, err := a.setupInputSubscription()
	if err != nil {
		return err
	}

	a.logger.Info("Waiting for input on NATS...", "input_subject", a.cfg.NatsInputSubject, "output_subject", a.cfg.NatsOutputSubject)

	// Set up signal handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	<-stop

	a.logger.Info("Shutting down gracefully...")

	if err := sub.Unsubscribe(); err != nil {
		a.logger.Error("Error unsubscribing from NATS", "error", err)
	}

	if err := a.nc.Drain(); err != nil {
		a.logger.Error("Error draining NATS connection", "error", err)
	}

	a.nc.Close()

	a.logger.Info("Shutdown complete")
	return nil
}

func (a *App) setupStreams() error {
	streams := []struct {
		name     string
		subjects []string
	}{
		{a.cfg.NatsInputStream, []string{a.cfg.NatsInputSubject}},
		{a.cfg.NatsOutputStream, []string{a.cfg.NatsOutputSubject}},
	}

	for _, s := range streams {
		_, err := a.js.AddStream(&nats.StreamConfig{
			Name:     s.name,
			Subjects: s.subjects,
		})
		if err != nil {
			return fmt.Errorf("error creating stream %s: %w", s.name, err)
		}
	}

	return nil
}

func (a *App) setupInputSubscription() (*nats.Subscription, error) {
	return a.js.Subscribe(a.cfg.NatsInputSubject, a.handleInput)
}

func (a *App) handleInput(m *nats.Msg) {
	var input map[string]interface{}
	if err := json.Unmarshal(m.Data, &input); err != nil {
		a.logger.Error("Error parsing input", "error", err)
		a.sendInputAck(m, false, "Error parsing input")
		return
	}

	policies := a.ruleEngine.GetAllPolicies()
	results := make([]api.PolicyResult, 0, len(policies))

	for _, policy := range policies {
		shouldExecute, err := policy.ShouldExecute(input)
		if err != nil {
			a.logger.Error("Error evaluating CEL expression", "error", err, "policy_id", policy.ID)
			results = append(results, api.PolicyResult{PolicyID: policy.ID, Result: false, Error: err.Error()})
			break
		}

		if !shouldExecute {
			a.logger.Debug("Policy will not be executed for this input", "policy_id", policy.ID)
			continue
		}

		result, err := a.ruleEngine.EvaluatePolicy(policy.ID, input)
		if err != nil {
			a.logger.Error("Error evaluating policy", "error", err, "policy_id", policy.ID)
			results = append(results, api.PolicyResult{PolicyID: policy.ID, Result: false, Error: err.Error()})
			break
		}

		results = append(results, api.PolicyResult{PolicyID: policy.ID, Result: result})
		if !result {
			break
		}

		a.logger.Info("Policy result", "policy_id", policy.ID, "result", result)
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		a.logger.Error("Error marshaling results", "error", err)
		a.sendInputAck(m, false, "Error marshaling results")
		return
	}

	if _, err := a.js.Publish(a.cfg.NatsOutputSubject, resultsJSON); err != nil {
		a.logger.Error("Error publishing results", "error", err)
		a.sendInputAck(m, false, "Error publishing results")
		return
	}

	a.sendInputAck(m, true, "Input processed successfully")
}

func (a *App) sendInputAck(m *nats.Msg, success bool, message string) {
	ack := api.InputAck{
		Success: success,
		Message: message,
	}
	ackJSON, err := json.Marshal(ack)
	if err != nil {
		a.logger.Error("Error marshaling acknowledgment", "error", err)
		return
	}
	if err := m.Ack(); err != nil {
		a.logger.Error("Error acknowledging message", "error", err)
	}
	if _, err := a.js.Publish(m.Reply, ackJSON); err != nil {
		a.logger.Error("Error sending acknowledgment", "error", err)
	}
}
