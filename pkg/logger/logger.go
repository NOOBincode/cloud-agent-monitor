// Package logger provides structured logging with zap.
package logger

import (
	"context"
	"log/slog"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.SugaredLogger with context support.
type Logger struct {
	*zap.SugaredLogger
}

// ctxKey is the context key for logger.
type ctxKey struct{}

var (
	// Default logger instance.
	defaultLogger *Logger
	// RequestIDKey is the key for request ID in context.
	RequestIDKey = "request_id"
)

// Config holds logger configuration.
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json or console
}

// New creates a new logger instance.
func New(cfg Config) (*Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	var encoder zapcore.Encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	if cfg.Format == "console" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return &Logger{zapLogger.Sugar()}, nil
}

// SetDefault sets the default logger instance.
func SetDefault(l *Logger) {
	defaultLogger = l
}

// Default returns the default logger instance.
func Default() *Logger {
	if defaultLogger == nil {
		l, _ := New(Config{Level: "info", Format: "json"})
		defaultLogger = l
	}
	return defaultLogger
}

// WithContext returns a logger with context values.
func WithContext(ctx context.Context) *Logger {
	l := Default()
	if ctx == nil {
		return l
	}

	if reqID, ok := ctx.Value(ctxKey{}).(string); ok && reqID != "" {
		return &Logger{l.With(RequestIDKey, reqID)}
	}
	return l
}

// WithFields returns a logger with additional fields.
func (l *Logger) WithFields(fields ...interface{}) *Logger {
	return &Logger{l.With(fields...)}
}

// WithError returns a logger with error field.
func (l *Logger) WithError(err error) *Logger {
	return &Logger{l.With("error", err)}
}

// ToSlog returns a slog.Logger adapter for compatibility.
func (l *Logger) ToSlog() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

// Sync flushes any buffered log entries.
func (l *Logger) Sync() error {
	return l.SugaredLogger.Sync()
}

func Info(msg string, fields ...interface{}) {
	Default().Infof(msg, fields...)
}

func Error(msg string, fields ...interface{}) {
	Default().Errorf(msg, fields...)
}

func Warn(msg string, fields ...interface{}) {
	Default().Warnf(msg, fields...)
}

func Debug(msg string, fields ...interface{}) {
	Default().Debugf(msg, fields...)
}
