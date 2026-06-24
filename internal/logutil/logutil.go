// Package logutil provides structured logging for ztutor.
// It wraps log/slog with severity-leveled helpers that accept
// printf-style format strings for minimal migration friction.
//
// Set LOG_LEVEL=debug to enable debug messages; default is info.
package logutil

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func init() {
	level := slog.LevelInfo
	if v := os.Getenv("LOG_LEVEL"); strings.EqualFold(v, "debug") {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))
}

// Debug logs at debug level (shown only when LOG_LEVEL=debug).
func Debug(msg string, args ...any) {
	slog.Debug(fmsg(msg, args...))
}

// Info logs at info level.
func Info(msg string, args ...any) {
	slog.Info(fmsg(msg, args...))
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	slog.Warn(fmsg(msg, args...))
}

// Error logs at error level.
func Error(msg string, args ...any) {
	slog.Error(fmsg(msg, args...))
}

// Fatal logs at error level and exits with code 1.
func Fatal(msg string, args ...any) {
	slog.Error(fmsg(msg, args...))
	os.Exit(1)
}

func fmsg(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}
