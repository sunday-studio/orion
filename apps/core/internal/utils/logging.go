package utils

import (
	"io"
	"log/slog"
	"os"
	"sync"
)

// Logger wraps slog.Logger for structured logging
type Logger struct {
	*slog.Logger
	tee *teeWriter
}

// teeWriter writes every log line to the process output and, once attached, to a sink
// (Core records its own log lines for the Internal logs screen). A sink never blocks or fails a log call.
type teeWriter struct {
	out  io.Writer
	mu   sync.RWMutex
	sink io.Writer
}

func (t *teeWriter) Write(p []byte) (int, error) {
	n, err := t.out.Write(p)
	t.mu.RLock()
	sink := t.sink
	t.mu.RUnlock()
	if sink != nil {
		_, _ = sink.Write(append([]byte(nil), p...))
	}
	return n, err
}

// AttachSink starts copying log lines to w. Pass nil to stop.
func (l *Logger) AttachSink(w io.Writer) {
	if l.tee == nil {
		return
	}
	l.tee.mu.Lock()
	l.tee.sink = w
	l.tee.mu.Unlock()
}

// NewLogger creates a new structured logger
func NewLogger() *Logger {
	return NewLoggerWithWriter(os.Stdout)
}

// NewLoggerWithWriter builds the JSON logger on an arbitrary writer (tests capture output with it).
func NewLoggerWithWriter(w io.Writer) *Logger {
	// Create a JSON handler for structured logging
	tee := &teeWriter{out: w}
	handler := slog.NewJSONHandler(tee, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	return &Logger{
		Logger: slog.New(handler),
		tee:    tee,
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
