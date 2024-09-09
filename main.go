package main

import (
	"log/slog"
	"os"

	"github.com/sandrolain/rules/app"
)

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		slog.Error("Error loading configuration", "error", err)
		os.Exit(1)
	}

	application, err := app.NewApp(cfg)
	if err != nil {
		slog.Error("Error creating application", "error", err)
		os.Exit(1)
	}

	if err := application.Run(); err != nil {
		slog.Error("Error running application", "error", err)
		os.Exit(1)
	}
}
