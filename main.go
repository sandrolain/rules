package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/nats-io/nats.go"
	"github.com/sandrolain/rules/api"
	"github.com/sandrolain/rules/engine"
	"google.golang.org/grpc"
)

type Config struct {
	NatsURL     string `env:"NATS_URL" envDefault:"nats://localhost:4222" validate:"required,url"`
	NatsSubject string `env:"NATS_SUBJECT" envDefault:"input" validate:"required"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
	GRPCPort    string `env:"GRPC_PORT" envDefault:"50051" validate:"required"`
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

	ruleEngine, err := engine.NewRuleEngine()
	if err != nil {
		slog.Error("Error creating rule engine", "error", err)
		os.Exit(1)
	}

	// Set up gRPC server
	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("Failed to listen for gRPC", "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	rulesService := api.NewRulesServiceServer(ruleEngine)
	api.RegisterRulesServiceServer(grpcServer, rulesService)

	go func() {
		slog.Info("Starting gRPC server", "port", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("Failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("Error connecting to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	sub, err := nc.Subscribe(cfg.NatsSubject, func(m *nats.Msg) {
		var input map[string]interface{}
		if err := json.Unmarshal(m.Data, &input); err != nil {
			slog.Error("Error parsing input", "error", err)
			return
		}

		policies := ruleEngine.GetAllPolicies()
		for _, policy := range policies {
			shouldExecute, err := policy.ShouldExecute(input)
			if err != nil {
				slog.Error("Error evaluating CEL expression", "error", err, "policy_id", policy.ID)
				continue
			}

			if !shouldExecute {
				slog.Debug("Policy will not be executed for this input", "policy_id", policy.ID)
				continue
			}

			result, err := ruleEngine.EvaluatePolicy(policy.ID, input)
			if err != nil {
				slog.Error("Error evaluating policy", "error", err, "policy_id", policy.ID)
				continue
			}

			slog.Info("Policy result", "policy_id", policy.ID, "result", result)
		}
	})

	if err != nil {
		slog.Error("Error subscribing to NATS topic", "error", err)
		os.Exit(1)
	}

	// Set up signal handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	slog.Info("Waiting for input on NATS...", "subject", cfg.NatsSubject)

	// Wait for termination signal
	<-stop

	slog.Info("Shutting down gracefully...")

	// Stop gRPC server
	grpcServer.GracefulStop()

	// Unsubscribe and close connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	natsClosed := make(chan bool)

	nc.SetClosedHandler(func(conn *nats.Conn) {
		natsClosed <- true
	})

	if err := sub.Unsubscribe(); err != nil {
		slog.Error("Error unsubscribing from NATS", "error", err)
	}

	if err := nc.Drain(); err != nil {
		slog.Error("Error draining NATS connection", "error", err)
	}

	select {
	case <-natsClosed:
		cancel()
		slog.Info("NATS connection closed")
	case <-ctx.Done():
		slog.Warn("Timeout waiting for NATS connection to close")
	}

	slog.Info("Shutdown complete")
}
