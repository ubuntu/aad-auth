// Package logger is the package which is able to plug multiple sublogger (pam, nss, stderr…) .
package logger

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

// Logger is the interface used to access the logger.
type Logger interface {
	// Debug sends a debug level message to the logger
	Debug(format string, a ...any)
	// Info sends an informational message to the logger
	Info(format string, a ...any)
	// Warn sends a warning level message to the logger
	Warn(format string, a ...any)
	// Err sends an error level message to the logger
	Err(format string, a ...any)
	// Crit sends a critical message to the logger
	Crit(format string, a ...any)
	// Close closes the underlying logger
	Close() error
}

type ctxKey string

const (
	ctxloggerKey ctxKey = "loggerCtxKey"
)

// CtxWithLogger returns a new context with a logger embedeed.
func CtxWithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, ctxloggerKey, logger)
}

// CloseLoggerFromContext closes an underlying logger attached to context.
func CloseLoggerFromContext(ctx context.Context) error {
	l, ok := ctx.Value(ctxloggerKey).(Logger)
	if !ok {
		err := errors.New("no logger attached to context")
		log.Printf("ERROR: %v", err)
		return err
	}
	return l.Close()
}

// Debug calls the corresponding logger Debug() func from context.
func Debug(ctx context.Context, format string, a ...any) {
	msg := normalizeMsg(fmt.Sprintf(format, a...))
	l, ok := ctx.Value(ctxloggerKey).(Logger)
	if !ok {
		log.Printf("DEBUG: %v", msg)
		return
	}
	l.Debug(msg)
}

// Info calls the corresponding logger Info() func from context.
func Info(ctx context.Context, format string, a ...any) {
	msg := normalizeMsg(fmt.Sprintf(format, a...))
	l, ok := ctx.Value(ctxloggerKey).(Logger)
	if !ok {
		log.Printf("INFO: %v", msg)
		return
	}
	l.Info(msg)
}

// Warn calls the corresponding logger Warn() func from context.
func Warn(ctx context.Context, format string, a ...any) {
	msg := normalizeMsg(fmt.Sprintf(format, a...))
	l, ok := ctx.Value(ctxloggerKey).(Logger)
	if !ok {
		log.Printf("WARNING: %v", msg)
		return
	}
	l.Warn(msg)
}

// Err calls the corresponding logger Err() func from context.
func Err(ctx context.Context, format string, a ...any) {
	msg := normalizeMsg(fmt.Sprintf(format, a...))
	l, ok := ctx.Value(ctxloggerKey).(Logger)
	if !ok {
		log.Printf("ERROR: %v", msg)
		return
	}
	l.Err(msg)
}

// Crit calls the corresponding logger Crit() func from context.
func Crit(ctx context.Context, format string, a ...any) {
	msg := normalizeMsg(fmt.Sprintf(format, a...))
	l, ok := ctx.Value(ctxloggerKey).(Logger)
	if !ok {
		log.Printf("CRITICAL: %v", msg)
		return
	}
	l.Crit(msg)
}

// normalizeMsg use format to expand a to it.
// Returned msg will always ends with an EOL.
func normalizeMsg(format string, a ...any) string {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	return fmt.Sprintf(format, a...)
}
