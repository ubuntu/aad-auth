// Package nss is the package which is pure Go code dealing with getent interactions.
package nss

import (
	"context"
	"fmt"
	"log/syslog"
	"os"

	"github.com/ubuntu/aad-auth/internal/logger"
)

const (
	nssLogEnv = "NSS_AAD_DEBUG"
)

type options struct {
	debug  bool
	writer logWriter
}

type logWriter interface {
	Debug(msg string) error
	Info(msg string) error
	Warning(msg string) error
	Err(msg string) error
	Crit(msg string) error
	Close() error
}

// Option represents the functional option passed to logger.
type Option func(*options)

// CtxWithSyslogLogger attach a logger to the context and set priority based on environment.
func CtxWithSyslogLogger(ctx context.Context, opts ...Option) context.Context {
	priority := syslog.LOG_INFO

	o := options{
		debug: false,
	}
	// applied options
	for _, opt := range opts {
		opt(&o)
	}
	if os.Getenv(nssLogEnv) != "" {
		o.debug = true
	}

	if os.Getenv(nssLogEnv) == "stderr" {
		return ctx
	}

	if o.debug {
		priority = syslog.LOG_DEBUG
	}

	nssLogger, err := newLogger(priority, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: can't find syslog to write to, defaulting to stderr: %v\n", err)
		return ctx
	}

	return logger.CtxWithLogger(ctx, nssLogger)
}

// Logger is the logger connected to syslog.
type Logger struct {
	w logWriter

	priority syslog.Priority
}

// newLogger returns a logger ready to log to syslog.
func newLogger(priority syslog.Priority, opts ...Option) (*Logger, error) {
	o := options{}
	// applied options
	for _, opt := range opts {
		opt(&o)
	}

	// We break the set default and override pattern of applying functional
	// options in order to avoid setting up a syslog connection when we
	// explicitly specify a writer to use. This is useful for testing.
	if o.writer == nil {
		var err error
		o.writer, err = syslog.New(syslog.LOG_DEBUG, "")
		if err != nil {
			return nil, fmt.Errorf("can't create nss logger: %w", err)
		}
	}

	l := &Logger{
		w:        o.writer,
		priority: priority,
	}

	l.Debug("NSS AAD DEBUG enabled\n")
	return l, nil
}

// Close closes the underlying syslog connection.
func (l Logger) Close() error {
	return l.w.Close()
}

// Debug sends a debug level message to the logger.
func (l Logger) Debug(format string, a ...any) {
	if l.priority < syslog.LOG_DEBUG {
		return
	}
	l.w.Debug(prefixWithNss(format, a...))
}

// Info sends an informational message to the logger.
func (l Logger) Info(format string, a ...any) {
	l.w.Info(prefixWithNss(format, a...))
}

// Warn sends a warning level message to the logger.
func (l Logger) Warn(format string, a ...any) {
	l.w.Warning(prefixWithNss(format, a...))
}

// Err sends an error level message to the logger.
func (l Logger) Err(format string, a ...any) {
	l.w.Err(prefixWithNss(format, a...))
}

// Crit sends a critical message to the logger.
func (l Logger) Crit(format string, a ...any) {
	l.w.Crit(prefixWithNss(format, a...))
}

// prefixWithNss prefix msg with NSS before calling NormalizeMsg.
func prefixWithNss(format string, a ...any) string {
	format = fmt.Sprintf("nss_aad: %v", format)
	return fmt.Sprintf(format, a...)
}
