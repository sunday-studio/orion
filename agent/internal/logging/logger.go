package logging

import (
    "fmt"
    "log"
    "os"
    "time"
)

type Level int

const (
    LevelDebug Level = iota
    LevelInfo
    LevelWarn
    LevelError
    LevelFatal
)

// ANSI color codes for each log level
var levelColors = map[Level]string{
    LevelDebug: "\033[36m", // Cyan
    LevelInfo:  "\033[32m", // Green
    LevelWarn:  "\033[33m", // Yellow
    LevelError: "\033[31m", // Red
    LevelFatal: "\033[35m", // Magenta
}

const colorReset = "\033[0m"

var currentLevel = LevelInfo

var logger = log.New(os.Stdout, "", 0)

func SetLevel(l Level) {
    currentLevel = l
}

func logf(level Level, label string, msg string, args ...any) {
    if level < currentLevel {
        return
    }
    timestamp := time.Now().Format("2006-01-02 15:04:05")
    formatted := fmt.Sprintf(msg, args...)
    color := levelColors[level]
    coloredLabel := fmt.Sprintf("%s%s%s", color, label, colorReset)
    coloredMessage := fmt.Sprintf("%s%s%s", color, formatted, colorReset)
    // Timestamp does not use color for readability
    logger.Printf("[%s] [%s] %s", timestamp, coloredLabel, coloredMessage)
}

func Debugf(msg string, args ...any) { logf(LevelDebug, "DEBUG", msg, args...) }
func Infof(msg string, args ...any)  { logf(LevelInfo, "INFO", msg, args...) }
func Warnf(msg string, args ...any)  { logf(LevelWarn, "WARN", msg, args...) }
func Errorf(msg string, args ...any) { logf(LevelError, "ERROR", msg, args...) }

func Fatalf(msg string, args ...any) {
    logf(LevelFatal, "FATAL", msg, args...)
    os.Exit(1)
}
