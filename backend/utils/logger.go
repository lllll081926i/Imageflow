package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// Logger provides logging functionality
type Logger struct {
	level      LogLevel
	fileLogger *log.Logger
	file       *os.File
}

// NewLogger creates a new logger instance
func NewLogger(level LogLevel, enableFile bool) (*Logger, error) {
	logger := &Logger{
		level: level,
	}

	if enableFile {
		// Create logs directory
		logsDir := "logs"
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create logs directory: %w", err)
		}

		// Create log file with timestamp
		timestamp := time.Now().Format("20060102_150405")
		logPath := filepath.Join(logsDir, fmt.Sprintf("imageflow_%s.log", timestamp))

		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		logger.file = file
		logger.fileLogger = log.New(file, "", log.LstdFlags)
	}

	return logger, nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= DebugLevel {
		l.log("DEBUG", format, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= InfoLevel {
		l.log("INFO", format, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level <= WarnLevel {
		l.log("WARN", format, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	if l.level <= ErrorLevel {
		l.log("ERROR", format, args...)
	}
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log("FATAL", format, args...)
	os.Exit(1)
}

// log writes a log message to console and file
func (l *Logger) log(level string, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s", level, message)

	// Log to console
	fmt.Println(logLine)

	// Log to file if enabled
	if l.fileLogger != nil {
		l.fileLogger.Println(logLine)
	}
}
