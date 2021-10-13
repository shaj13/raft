package log

import (
	"log"
	"os"

	"go.etcd.io/etcd/raft/v3"
)

var lg Logger = &raft.DefaultLogger{Logger: log.New(os.Stderr, "", log.LstdFlags)}

// Logger represents an active logging object that generates lines of
// output to an io.Writer.
type Logger interface {
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})

	Error(v ...interface{})
	Errorf(format string, v ...interface{})

	Info(v ...interface{})
	Infof(format string, v ...interface{})

	Warning(v ...interface{})
	Warningf(format string, v ...interface{})

	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})

	Panic(v ...interface{})
	Panicf(format string, v ...interface{})
}

// Debug uses fmt.Sprint to construct and log a message.
func Debug(args ...interface{}) {
	lg.Debug(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(format string, args ...interface{}) {
	lg.Debugf(format, args...)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...interface{}) {
	lg.Info(args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(format string, args ...interface{}) {
	lg.Infof(format, args...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	lg.Warning(args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(format string, args ...interface{}) {
	lg.Warningf(format, args...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	lg.Error(args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(format string, args ...interface{}) {
	lg.Errorf(format, args...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...interface{}) {
	lg.Fatal(args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(format string, args ...interface{}) {
	lg.Fatalf(format, args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...interface{}) {
	lg.Panic(args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(format string, args ...interface{}) {
	lg.Panicf(format, args...)
}

// SetLogger the logger used by the package-level output functions.
func SetLogger(l Logger) {
	lg = l
}

// GetLogger returns the logger used by the package-level output functions.
func GetLogger() Logger {
	return lg
}
