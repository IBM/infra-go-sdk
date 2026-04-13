package svc

import (
	"io"
	"os"

	"github.com/charmbracelet/log"
)

// Logger wraps charmbracelet/log for structured logging
type Logger struct {
	*log.Logger
}

// Debug logs a debug message with key-value pairs
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.Logger.Debug(msg, keysAndValues...)
}

// Info logs an info message with key-value pairs
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.Logger.Info(msg, keysAndValues...)
}

// Warn logs a warning message with key-value pairs
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.Logger.Warn(msg, keysAndValues...)
}

// Error logs an error message with key-value pairs
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.Logger.Error(msg, keysAndValues...)
}

// NewLogger creates a new logger instance
func NewLogger(level log.Level, output io.Writer) *Logger {
	if output == nil {
		output = os.Stderr
	}

	l := log.NewWithOptions(output, log.Options{
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
		ReportCaller:    false,
	})

	l.SetLevel(level)

	return &Logger{
		Logger: l,
	}
}

// NewDefaultLogger creates a logger with default settings (Warn level, stderr)
func NewDefaultLogger() *Logger {
	return NewLogger(log.WarnLevel, os.Stderr)
}

// NewDebugLogger creates a logger with debug level enabled
func NewDebugLogger() *Logger {
	return NewLogger(log.DebugLevel, os.Stderr)
}