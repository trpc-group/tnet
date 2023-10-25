// Package log provides logging utilities for tnet.
package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Default borrows logging utilities from zap.
// The default log level is info level.
// The default output is standard output.
// You may replace it with whatever logger you like as long as it implements log.Logger interface.
var Default Logger = zap.New(
	zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zap.NewAtomicLevelAt(zapcore.InfoLevel),
	),
	zap.AddCaller(),
	zap.AddCallerSkip(1),
).Sugar()

var encoderConfig = zapcore.EncoderConfig{
	TimeKey:        "ts",
	LevelKey:       "lvl",
	NameKey:        "name",
	CallerKey:      "caller",
	MessageKey:     "message",
	StacktraceKey:  "stacktrace",
	LineEnding:     zapcore.DefaultLineEnding,
	EncodeLevel:    zapcore.CapitalColorLevelEncoder,
	EncodeTime:     zapcore.RFC3339TimeEncoder,
	EncodeDuration: zapcore.SecondsDurationEncoder,
	EncodeCaller:   zapcore.ShortCallerEncoder,
}

// Logger provides a unified logging interface.
type Logger interface {
	// Debug logs to DEBUG log. Arguments are handled in the manner of fmt.Print.
	Debug(args ...any)
	// Debugf logs to DEBUG log. Arguments are handled in the manner of fmt.Printf.
	Debugf(format string, args ...any)
	// Info logs to INFO log. Arguments are handled in the manner of fmt.Print.
	Info(args ...any)
	// Infof logs to INFO log. Arguments are handled in the manner of fmt.Printf.
	Infof(format string, args ...any)
	// Warn logs to WARNING log. Arguments are handled in the manner of fmt.Print.
	Warn(args ...any)
	// Warnf logs to WARNING log. Arguments are handled in the manner of fmt.Printf.
	Warnf(format string, args ...any)
	// Error logs to ERROR log. Arguments are handled in the manner of fmt.Print.
	Error(args ...any)
	// Errorf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
	Errorf(format string, args ...any)
	// Fatal logs to ERROR log. Arguments are handled in the manner of fmt.Print.
	Fatal(args ...any)
	// Fatalf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
	Fatalf(format string, args ...any)
}

// Debug logs to DEBUG log. Arguments are handled in the manner of fmt.Print.
func Debug(args ...any) {
	Default.Debug(args...)
}

// Debugf logs to DEBUG log. Arguments are handled in the manner of fmt.Printf.
func Debugf(format string, args ...any) {
	Default.Debugf(format, args...)
}

// Info logs to INFO log. Arguments are handled in the manner of fmt.Print.
func Info(args ...any) {
	Default.Info(args...)
}

// Infof logs to INFO log. Arguments are handled in the manner of fmt.Printf.
func Infof(format string, args ...any) {
	Default.Infof(format, args...)
}

// Warn logs to WARNING log. Arguments are handled in the manner of fmt.Print.
func Warn(args ...any) {
	Default.Warn(args...)
}

// Warnf logs to WARNING log. Arguments are handled in the manner of fmt.Printf.
func Warnf(format string, args ...any) {
	Default.Warnf(format, args...)
}

// Error logs to ERROR log. Arguments are handled in the manner of fmt.Print.
func Error(args ...any) {
	Default.Error(args...)
}

// Errorf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
func Errorf(format string, args ...any) {
	Default.Errorf(format, args...)
}

// Fatal logs to ERROR log. Arguments are handled in the manner of fmt.Print.
func Fatal(args ...any) {
	Default.Fatal(args...)
}

// Fatalf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
func Fatalf(format string, args ...any) {
	Default.Fatalf(format, args...)
}
