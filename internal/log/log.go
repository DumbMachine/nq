package log

import (
	"fmt"
	"io"
	defualtLog "log"
	"os"
)

// Base supports logging at various log levels.
type Base interface {
	// Debug logs a message at Debug level.
	Debug(args ...interface{})

	// Info logs a message at Info level.
	Info(args ...interface{})

	// Warn logs a message at Warning level.
	Warn(args ...interface{})

	// Error logs a message at Error level.
	Error(args ...interface{})

	// Fatal logs a message at Fatal level
	// and process will exit with status set to 1.
	Fatal(args ...interface{})
}

// baseLogger is a wrapper object around log.Logger from the standard library.
// It supports logging at various log levels.
type baseLogger struct {
	*defualtLog.Logger
}

// Debug logs a message at Debug level.
func (l *baseLogger) Debug(args ...interface{}) {
	l.printWithPrefix("DEBUG: ", args...)
}

// Info logs a message at Info level.
func (l *baseLogger) Info(args ...interface{}) {
	l.printWithPrefix("INFO: ", args...)
}

// Warn logs a message at Warning level.
func (l *baseLogger) Warn(args ...interface{}) {
	l.printWithPrefix("WARN: ", args...)
}

// Error logs a message at Error level.
func (l *baseLogger) Error(args ...interface{}) {
	l.printWithPrefix("ERROR: ", args...)
}

// Fatal logs a message at Fatal level
// and process will exit with status set to 1.
func (l *baseLogger) Fatal(args ...interface{}) {
	l.printWithPrefix("FATAL: ", args...)
	os.Exit(1)
}

func (l *baseLogger) printWithPrefix(prefix string, args ...interface{}) {
	args = append([]interface{}{prefix}, args...)
	l.Print(args...)
}

// newBase creates and returns a new instance of baseLogger.
func newBase(out io.Writer) *baseLogger {
	prefix := fmt.Sprintf("nq: pid=%d ", os.Getpid())
	return &baseLogger{
		defualtLog.New(out, prefix, defualtLog.Ldate|defualtLog.Ltime|defualtLog.Lmicroseconds|defualtLog.LUTC),
	}
}

// Level represents a log level.
type Level int32

const (
	// DebugLevel is the lowest level of logging.
	// Debug logs are intended for debugging and development purposes.
	DebugLevel Level = iota

	// InfoLevel is used for general informational log messages.
	InfoLevel

	// WarnLevel is used for undesired but relatively expected events,
	// which may indicate a problem.
	WarnLevel

	// ErrorLevel is used for undesired and unexpected events that
	// the program can recover from.
	ErrorLevel

	// FatalLevel is used for undesired and unexpected events that
	// the program cannot recover from.
	FatalLevel
)

// String is part of the fmt.Stringer interface.
//
// Used for testing and debugging purposes.
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warning"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	default:
		return "unknown"
	}
}

// Logger logs message to io.Writer at various log levels.
type Logger struct {
	base Base
	// Minimum log level for this logger.
	// Message with level lower than this level won't be outputted.
	level Level
}

// NewLogger creates and returns a new instance of Logger set to appropriate level.
//
// Log level is set to DebugLevel by default.
func NewLogger(base Base, logLevel Level) *Logger {
	if base == nil {
		base = newBase(os.Stderr)
	}
	logg := &Logger{base: base, level: DebugLevel}
	logg.SetLevel(logLevel)
	return logg
}

// canLogAt reports whether logger can log at level v.
func (l *Logger) canLogAt(v Level) bool {
	return v >= l.level
}

func (l *Logger) Debug(args ...interface{}) {
	if !l.canLogAt(DebugLevel) {
		return
	}
	l.base.Debug(args...)
}

func (l *Logger) Info(args ...interface{}) {
	if !l.canLogAt(InfoLevel) {
		return
	}
	l.base.Info(args...)
}

func (l *Logger) Warn(args ...interface{}) {
	if !l.canLogAt(WarnLevel) {
		return
	}
	l.base.Warn(args...)
}

func (l *Logger) Error(args ...interface{}) {
	if !l.canLogAt(ErrorLevel) {
		return
	}
	l.base.Error(args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	if !l.canLogAt(FatalLevel) {
		return
	}
	l.base.Fatal(args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Fatal(fmt.Sprintf(format, args...))
}

// SetLevel sets the logger level.
// It panics if v is less than DebugLevel or greater than FatalLevel.
func (l *Logger) SetLevel(v Level) {
	if v < DebugLevel || v > FatalLevel {
		panic("log: invalid log level")
	}
	l.level = v
}
