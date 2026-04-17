// Package observability provides logging, metrics, and tracing utilities.
package observability

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the application logger.
type Logger struct {
	*zap.Logger
}

// Config contains logger configuration.
type LoggerConfig struct {
	Level      string
	Format     string // "json" or "console"
	OutputPath string
}

// NewLogger creates a new structured logger.
func NewLogger(cfg LoggerConfig) (*Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	var writeSyncer zapcore.WriteSyncer
	if cfg.OutputPath == "" || cfg.OutputPath == "stdout" {
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else {
		file, err := os.OpenFile(cfg.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		writeSyncer = zapcore.AddSync(file)
	}

	core := zapcore.NewCore(encoder, writeSyncer, level)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{logger}, nil
}

// NewDevelopmentLogger creates a logger for development.
func NewDevelopmentLogger() *Logger {
	logger, err := NewLogger(LoggerConfig{
		Level:  "debug",
		Format: "console",
	})
	if err != nil {
		panic(err)
	}
	return logger
}

// NewProductionLogger creates a logger for production.
func NewProductionLogger() *Logger {
	logger, err := NewLogger(LoggerConfig{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		panic(err)
	}
	return logger
}

// WithSession returns a logger with session context fields.
func (l *Logger) WithSession(sessionID, traceID string) *Logger {
	return &Logger{l.Logger.With(
		zap.String("session_id", sessionID),
		zap.String("trace_id", traceID),
	)}
}

// WithContext returns a logger with context fields.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract any context values and add them as fields
	fields := []zap.Field{}

	if sessionID, ok := ctx.Value("session_id").(string); ok {
		fields = append(fields, zap.String("session_id", sessionID))
	}
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if tenantID, ok := ctx.Value("tenant_id").(string); ok {
		fields = append(fields, zap.String("tenant_id", tenantID))
	}

	return &Logger{l.Logger.With(fields...)}
}

// WithField returns a logger with an additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{l.Logger.With(zap.Any(key, value))}
}

// WithFields returns a logger with additional fields.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return &Logger{l.Logger.With(zapFields...)}
}

// WithError returns a logger with an error field.
func (l *Logger) WithError(err error) *Logger {
	return &Logger{l.Logger.With(zap.Error(err))}
}

// Sync flushes any buffered log entries.
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// LogLevel returns the current log level.
func (l *Logger) LogLevel() zapcore.Level {
	return l.Logger.Level()
}

// IsDebugEnabled returns true if debug logging is enabled.
func (l *Logger) IsDebugEnabled() bool {
	return l.Logger.Core().Enabled(zapcore.DebugLevel)
}

// NoopLogger returns a logger that discards all output.
func NoopLogger() *Logger {
	return &Logger{zap.NewNop()}
}

// contextKey is used for storing logger in context.
type contextKey struct{}

var loggerKey = &contextKey{}

// ContextWithLogger adds a logger to the context.
func ContextWithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext retrieves the logger from context.
// Returns a noop logger if not found.
func LoggerFromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}
	return NoopLogger()
}
