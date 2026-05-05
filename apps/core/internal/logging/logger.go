package logging

import (
	"log/slog"
	"os"
)

// Logger wraps slog.Logger for structured logging
type Logger struct {
	*slog.Logger
}

// NewLogger creates a new structured logger
func NewLogger() *Logger {
	// Create a JSON handler for structured logging
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	return &Logger{
		Logger: slog.New(handler),
	}
}

// Info logs an info message with optional key-value pairs
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// Error logs an error message with optional key-value pairs
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

// Fatal logs a fatal error and exits the program
func (l *Logger) Fatal(msg string, args ...any) {
	l.Logger.Error(msg, args...)
	os.Exit(1)
}

// Debug logs a debug message with optional key-value pairs
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

// Warn logs a warning message with optional key-value pairs
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}
