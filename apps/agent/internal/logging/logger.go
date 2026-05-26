package logging

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

type FileConfig struct {
	Path       string
	Level      Level
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

var levelColors = map[Level]string{
	LevelDebug: "\033[36m",
	LevelInfo:  "\033[32m",
	LevelWarn:  "\033[33m",
	LevelError: "\033[31m",
	LevelFatal: "\033[35m",
}

const colorReset = "\033[0m"

var (
	mu           sync.Mutex
	currentLevel = LevelInfo
	textLogger   = log.New(os.Stdout, "", 0)
	jsonLogger   *slog.Logger
	levelVar     slog.LevelVar
	fileCloser   io.Closer
	fileMode     bool
	textColor    = true
)

func init() {
	levelVar.Set(slogLevel(LevelInfo))
}

func ParseLevel(level string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return LevelDebug, nil
	case "", "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unsupported log level %q", level)
	}
}

func SetLevel(l Level) {
	mu.Lock()
	defer mu.Unlock()

	currentLevel = l
	levelVar.Set(slogLevel(l))
}

func ConfigureText(out io.Writer) {
	mu.Lock()
	defer mu.Unlock()

	closeFileLocked()
	textLogger = log.New(out, "", 0)
	jsonLogger = nil
	fileMode = false
	log.SetOutput(out)
	log.SetFlags(0)
}

func SetTextColorEnabled(enabled bool) {
	mu.Lock()
	defer mu.Unlock()

	textColor = enabled
}

func ConfigureFile(cfg FileConfig) error {
	if strings.TrimSpace(cfg.Path) == "" {
		return fmt.Errorf("log path is required")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o750); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	logFile, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	if err := logFile.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	if err := os.Chmod(cfg.Path, 0o640); err != nil {
		return fmt.Errorf("secure log file permissions: %w", err)
	}

	writer := &lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
		LocalTime:  true,
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: &levelVar})
	logger := slog.New(handler)
	stdLogger := slog.NewLogLogger(handler, slog.LevelInfo)

	mu.Lock()
	defer mu.Unlock()

	closeFileLocked()
	currentLevel = cfg.Level
	levelVar.Set(slogLevel(cfg.Level))
	jsonLogger = logger
	fileCloser = writer
	fileMode = true
	log.SetOutput(stdLogger.Writer())
	log.SetFlags(0)
	slog.SetDefault(logger)

	return nil
}

func Close() error {
	mu.Lock()
	defer mu.Unlock()

	return closeFileLocked()
}

func logf(level Level, label string, msg string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	if level < currentLevel {
		return
	}

	formatted := fmt.Sprintf(msg, args...)
	if fileMode && jsonLogger != nil {
		jsonLogger.LogAttrs(
			context.Background(),
			slogLevel(level),
			formatted,
			slog.String("component", "agent"),
		)
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelColor := ""
	reset := ""
	if textColor {
		levelColor = levelColors[level]
		reset = colorReset
	}
	coloredLabel := fmt.Sprintf("%s%s%s", levelColor, label, reset)
	coloredMessage := fmt.Sprintf("%s%s%s", levelColor, formatted, reset)
	textLogger.Printf("[%s] [%s] %s", timestamp, coloredLabel, coloredMessage)
}

func Debugf(msg string, args ...any) { logf(LevelDebug, "DEBUG", msg, args...) }
func Infof(msg string, args ...any)  { logf(LevelInfo, "INFO", msg, args...) }
func Warnf(msg string, args ...any)  { logf(LevelWarn, "WARN", msg, args...) }
func Errorf(msg string, args ...any) { logf(LevelError, "ERROR", msg, args...) }

func Fatalf(msg string, args ...any) {
	logf(LevelFatal, "FATAL", msg, args...)
	os.Exit(1)
}

func slogLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError, LevelFatal:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func closeFileLocked() error {
	if fileCloser == nil {
		return nil
	}
	err := fileCloser.Close()
	fileCloser = nil
	return err
}
