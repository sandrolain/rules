package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/protobuf/proto"

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
		// Check if the stream already exists
		stream, err := a.js.StreamInfo(s.name)
		if err != nil && err != nats.ErrStreamNotFound {
			return fmt.Errorf("error retrieving stream info for %s: %w", s.name, err)
		}

		if stream == nil {
			// The stream doesn't exist, create it
			_, err = a.js.AddStream(&nats.StreamConfig{
				Name:      s.name,
				Subjects:  s.subjects,
				Retention: nats.InterestPolicy,
			})
			if err != nil {
				return fmt.Errorf("error creating stream %s: %w", s.name, err)
			}
			a.logger.Info("Stream created", "name", s.name, "subjects", s.subjects)
		} else {
			// The stream already exists, check if we need to update it
			needsUpdate := false
			if !equalStringSlices(stream.Config.Subjects, s.subjects) {
				stream.Config.Subjects = s.subjects
				needsUpdate = true
			}
			if stream.Config.Retention != nats.InterestPolicy {
				stream.Config.Retention = nats.InterestPolicy
				needsUpdate = true
			}

			if needsUpdate {
				_, err = a.js.UpdateStream(&stream.Config)
				if err != nil {
					return fmt.Errorf("error updating stream %s: %w", s.name, err)
				}
				a.logger.Info("Stream updated", "name", s.name, "subjects", s.subjects)
			} else {
				a.logger.Info("Stream already exists and is correct", "name", s.name)
			}
		}
	}

	return nil
}

// Funzione di utilità per confrontare slice di stringhe
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
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
			results = append(results, api.PolicyResult{PolicyId: policy.ID, ResultThreshold: "", Error: err.Error()})
			break
		}

		if !shouldExecute {
			a.logger.Debug("Policy will not be executed for this input", "policy_id", policy.ID)
			continue
		}

		result, ruleResults, err := a.ruleEngine.EvaluatePolicy(policy.ID, input)
		if err != nil {
			a.logger.Error("Error evaluating policy", "error", err, "policy_id", policy.ID)
			results = append(results, api.PolicyResult{PolicyId: policy.ID, ResultThreshold: "", Error: err.Error()})
			break
		}

		apiRuleResults := make([]*api.RuleResult, len(ruleResults))
		for i, rr := range ruleResults {
			apiRuleResults[i] = &api.RuleResult{
				Score:    rr.Score,
				Stop:     rr.Stop,
				Executed: rr.Executed,
			}
		}

		results = append(results, api.PolicyResult{
			PolicyId:        policy.ID,
			ResultThreshold: result,
			RuleResults:     apiRuleResults,
		})

		a.logger.Info("Policy result", "policy_id", policy.ID, "result", result, "rule_results", ruleResults)
	}

	resultsPtr := make([]*api.PolicyResult, len(results))
	for i := range results {
		resultsPtr[i] = &results[i]
	}

	resultsProto := &api.PolicyResults{
		Results: resultsPtr,
	}
	resultsData, err := proto.Marshal(resultsProto)
	if err != nil {
		a.logger.Error("Error marshaling results", "error", err)
		a.sendInputAck(m, false, "Error marshaling results")
		return
	}

	if _, err := a.js.Publish(a.cfg.NatsOutputSubject, resultsData); err != nil {
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
