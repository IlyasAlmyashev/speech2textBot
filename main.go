// Command speech2textBot is a Telegram bot that transcribes voice messages.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		panic(err)
	}
	log := newLogger(cfg.LogLevel)
	b := newBot(cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Registered before listening: Cloud Run grants CPU during startup, but may
	// take it away once the port is open and no request is in progress. A
	// failure is not fatal, because the previous registration stays active.
	if err := b.telegram.setWebhook(ctx, cfg.BotURL+updatePath, cfg.WebhookSecret); err != nil {
		log.Error("webhook registration failed", "error", err)
	}

	listener, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		panic(err)
	}
	server := &http.Server{
		Handler:           b.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	log.Info("server started", "port", cfg.Port)

	select {
	case err := <-serveErr:
		log.Error("server stopped", "error", err)
		os.Exit(1)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("shutdown failed", "error", err)
	}
	log.Info("server stopped")
}

// newLogger writes JSON using the field names Cloud Logging understands.
func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) > 0 {
				return attr
			}
			switch attr.Key {
			case slog.LevelKey:
				attr.Key = "severity"
				if lvl, ok := attr.Value.Any().(slog.Level); ok && lvl == slog.LevelWarn {
					attr.Value = slog.StringValue("WARNING")
				}
			case slog.MessageKey:
				attr.Key = "message"
			}
			return attr
		},
	}))
}
