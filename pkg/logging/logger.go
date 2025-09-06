package logging

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
)

type Logger struct {
	*slog.Logger
}

type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// ContextKey for correlation IDs
type contextKey string

const correlationIDKey contextKey = "correlation_id"

func NewLogger(level LogLevel) *Logger {
	var slogLevel slog.Level
	switch level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &Logger{Logger: logger}
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context) context.Context {
	if GetCorrelationID(ctx) == "" {
		correlationID := uuid.New().String()
		return context.WithValue(ctx, correlationIDKey, correlationID)
	}
	return ctx
}

// GetCorrelationID retrieves the correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if correlationID, ok := ctx.Value(correlationIDKey).(string); ok {
		return correlationID
	}
	return ""
}

// Debug logs debug level messages with correlation ID
func (l *Logger) Debug(ctx context.Context, msg string, args ...any) {
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		args = append(args, "correlation_id", correlationID)
	}
	l.Logger.Debug(msg, args...)
}

// Info logs info level messages with correlation ID
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		args = append(args, "correlation_id", correlationID)
	}
	l.Logger.Info(msg, args...)
}

// Warn logs warn level messages with correlation ID
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		args = append(args, "correlation_id", correlationID)
	}
	l.Logger.Warn(msg, args...)
}

// Error logs error level messages with correlation ID
func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		args = append(args, "correlation_id", correlationID)
	}
	l.Logger.Error(msg, args...)
}

// LogLinkOperation logs link operations without sensitive data
func (l *Logger) LogLinkOperation(ctx context.Context, operation, code string, success bool) {
	correlationID := GetCorrelationID(ctx)
	l.Logger.Info("link operation",
		"operation", operation,
		"code", code,
		"success", success,
		"correlation_id", correlationID,
	)
}

// LogURLValidation logs URL validation without the actual URL
func (l *Logger) LogURLValidation(ctx context.Context, valid bool, scheme string) {
	correlationID := GetCorrelationID(ctx)
	l.Logger.Debug("url validation",
		"valid", valid,
		"scheme", scheme, // Safe to log scheme
		"correlation_id", correlationID,
	)
}

// LogAuthEvent logs authentication events without sensitive data
func (l *Logger) LogAuthEvent(ctx context.Context, event string, userID string, success bool) {
	correlationID := GetCorrelationID(ctx)
	// Hash the user ID to avoid logging PII while maintaining traceability
	hashedUserID := hashSensitiveData(userID)
	l.Logger.Info("auth event",
		"event", event,
		"user_hash", hashedUserID,
		"success", success,
		"correlation_id", correlationID,
	)
}

// Simple hash function for sensitive data logging
func hashSensitiveData(data string) string {
	if len(data) < 8 {
		return "***"
	}
	// Show first 3 and last 3 chars with stars in middle
	return data[:3] + "***" + data[len(data)-3:]
}
