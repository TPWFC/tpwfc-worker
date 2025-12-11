// Package logger provides logging utilities for the worker service.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Logger provides structured logging functionality.
type Logger struct {
	internal *slog.Logger
	level    *slog.LevelVar
}

// NewLogger creates a new logger instance with the specified level.
func NewLogger(level string) *Logger {
	lvl := new(slog.LevelVar)

	switch strings.ToLower(level) {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "info":
		lvl.Set(slog.LevelInfo)
	case "warn":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	default:
		lvl.Set(slog.LevelInfo)
	}

	opts := &slog.HandlerOptions{
		Level: lvl,
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	internal := slog.New(handler)

	return &Logger{
		internal: internal,
		level:    lvl,
	}
}

// Info logs an info level message.
func (l *Logger) Info(msg string, args ...any) {
	l.internal.Info(msg, args...)
}

// Error logs an error level message.
func (l *Logger) Error(msg string, args ...any) {
	l.internal.Error(msg, args...)
}

// Debug logs a debug level message.
func (l *Logger) Debug(msg string, args ...any) {
	l.internal.Debug(msg, args...)
}

// Warn logs a warning level message.
func (l *Logger) Warn(msg string, args ...any) {
	l.internal.Warn(msg, args...)
}

// With creates a child logger with the given attributes.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		internal: l.internal.With(args...),
		level:    l.level,
	}
}

// Log logs a message with the given level and attributes.
func (l *Logger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	l.internal.Log(ctx, level, msg, args...)
}
