// Package logger provides the process-wide structured logger.
// Rule (fintech-review #7): never log passwords, OTP codes, tokens, full
// phone numbers, or balances at info level or above.
package logger

import (
	"log/slog"
	"os"
)

// New returns a JSON slog logger. In dev, source locations are included.
func New(env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if env == "dev" {
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
