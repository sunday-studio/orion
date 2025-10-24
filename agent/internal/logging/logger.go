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
    logger.Printf("[%s] [%s] %s", timestamp, label, formatted)
}

func Debugf(msg string, args ...any) { logf(LevelDebug, "DEBUG", msg, args...) }
func Infof(msg string, args ...any)  { logf(LevelInfo, "INFO", msg, args...) }
func Warnf(msg string, args ...any)  { logf(LevelWarn, "WARN", msg, args...) }
func Errorf(msg string, args ...any) { logf(LevelError, "ERROR", msg, args...) }

func Fatalf(msg string, args ...any) {
    logf(LevelFatal, "FATAL", msg, args...)
    os.Exit(1)
}
