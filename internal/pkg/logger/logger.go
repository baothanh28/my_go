package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"myapp/internal/pkg/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger for structured logging
type Logger struct {
	*zap.Logger
}

// NewLogger creates a new logger instance based on configuration
func NewLogger(cfg *config.Config) (*Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create output paths
	// Support multiple paths separated by comma, or single path
	outputPaths := []string{}
	if cfg.Logger.OutputPath == "" {
		outputPaths = []string{"stdout"}
	} else {
		// Split by comma to support multiple outputs (e.g., "stdout,logs/app.log")
		paths := strings.Split(cfg.Logger.OutputPath, ",")
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path != "" && path != "stdout" && path != "stderr" {
				// For file paths, ensure directory exists
				dir := filepath.Dir(path)
				if dir != "." && dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return nil, fmt.Errorf("failed to create log directory: %w", err)
					}
				}
			}
			outputPaths = append(outputPaths, path)
		}
		// If no valid paths, default to stdout
		if len(outputPaths) == 0 {
			outputPaths = []string{"stdout"}
		}
	}

	// Create error output paths (errors go to stderr and error log file if file logging is enabled)
	errorOutputPaths := []string{"stderr"}
	for _, path := range outputPaths {
		if path != "stdout" && path != "stderr" {
			// Create error log file name (e.g., logs/app.log -> logs/app.error.log)
			ext := filepath.Ext(path)
			base := strings.TrimSuffix(path, ext)
			errorPath := base + ".error" + ext
			errorOutputPaths = append(errorOutputPaths, errorPath)
			break // Only add one error log file
		}
	}

	// Create logger config
	zapConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Encoding:         cfg.Logger.Format,
		EncoderConfig:    encoderConfig,
		OutputPaths:      outputPaths,
		ErrorOutputPaths: errorOutputPaths,
	}

	// Build logger
	zapLogger, err := zapConfig.Build(
		zap.AddCallerSkip(1),
		zap.AddCaller(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return &Logger{Logger: zapLogger}, nil
}

// Info logs an info level message
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

// Error logs an error level message
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

// Debug logs a debug level message
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

// Warn logs a warn level message
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

// Fatal logs a fatal level message and exits
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}

// With creates a child logger with the given fields
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{Logger: l.Logger.With(fields...)}
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}
