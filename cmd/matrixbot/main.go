package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mopore/matrix_http_api_go_example/internal/config"
	"github.com/mopore/matrix_http_api_go_example/internal/matrixapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := matrixapi.NewClient(
		matrixapi.WithHomeserver(cfg.Homeserver),
		matrixapi.WithAccessToken(cfg.BotAccessToken),
		matrixapi.WithRoomID(cfg.RoomID),
		matrixapi.WithHumanUserID(cfg.HumanUserID),
	)

	if err := client.Whoami(ctx); err != nil {
		return err
	}
	slog.Info("Logged in", "bot_id", client.BotUserID)

	var wg sync.WaitGroup
	startHeartbeat(ctx, &wg)

	if err := client.SendMessage(ctx, "Matrix Bot is online. Say something, jni."); err != nil {
		slog.Warn("Failed to send welcome message", "error", err)
	}

	slog.Info("Starting sync loop")
	if err := handleEvents(ctx, client, stop); err != nil {
		return err
	}

	slog.Info("Shutting down...")
	wg.Wait()
	slog.Info("End of main loop")
	return nil
}

func startHeartbeat(ctx context.Context, wg *sync.WaitGroup) {
	wg.Go(func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				i++
				slog.Info("Example count", "i", i)
			}
		}
	})
}

func handleEvents(ctx context.Context, client *matrixapi.Client, stop context.CancelFunc) error {
	for event, err := range client.Sync(ctx) {
		if err != nil {
			slog.Error("Sync error", "error", err)
			continue
		}

		slog.Info("Message received", "sender", event.Sender, "body", event.Content.Body)

		if strings.ToLower(strings.TrimSpace(event.Content.Body)) == "exit" {
			slog.Info("'exit' received from user")
			_ = client.SendMessage(ctx, "Received your 'exit'")
			stop() // Signal context cancellation
			return nil
		}

		err = client.SendMessage(ctx, "Ack: \""+event.Content.Body+"\"")
		if err != nil {
			slog.Error("Failed to send ack", "error", err)
		}
	}
	return nil
}
