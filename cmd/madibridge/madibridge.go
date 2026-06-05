package main

import (
	"log/slog"
	"os"

	"github.com/xorsirenz/madibridge/internal/bridge"
	"github.com/xorsirenz/madibridge/internal/config"
	"github.com/xorsirenz/madibridge/internal/utils"
)

var version string

func main() {
	if len(os.Args) > 1 {
		utils.HandleCmd(version)
	}

	logger := slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	)

	slog.SetDefault(logger)

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("failed to load config:", slog.Any("error", err))
		os.Exit(1)
	}

	bridge, err := bridge.New(cfg)
	if err != nil {
		logger.Error("failed to create bridge", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("Bridge running")

	if err := bridge.Run(); err != nil {
		logger.Error("bridge stopped", slog.Any("error", err))
		os.Exit(1)
	}
}
