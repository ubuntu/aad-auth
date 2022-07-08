package nss

import (
	"fmt"
	"log/syslog"

	"github.com/ubuntu/aad-auth/internal/logger"
)

// Logger is the logger connected to syslog.
type Logger struct {
	w *syslog.Writer

	priority syslog.Priority
}

// NewLogger returns a logger ready to log to syslog.
func NewLogger(priority syslog.Priority) (*Logger, error) {
	w, err := syslog.New(syslog.LOG_DEBUG, "")
	if err != nil {
		return nil, fmt.Errorf("can't create nss logger: %v", err)
	}

	l := &Logger{
		w:        w,
		priority: priority,
	}

	l.Debug("NSS AAD DEBUG enabled")
	return l, nil
}

// Close closes the underlying syslog connection
func (l Logger) Close() error {
	return l.w.Close()
}

// Debug sends a debug level message to the logger.
func (l Logger) Debug(format string, a ...any) {
	if l.priority < syslog.LOG_DEBUG {
		return
	}
	l.w.Debug(normalizeMsgAndPrefixNss(format, a...))
}

// Info sends an informational message to the logger.
func (l Logger) Info(format string, a ...any) {
	l.w.Info(normalizeMsgAndPrefixNss(format, a...))
}

// Warn sends a warning level message to the logger.
func (l Logger) Warn(format string, a ...any) {
	l.w.Warning(normalizeMsgAndPrefixNss(format, a...))
}

// Err sends an error level message to the logger.
func (l Logger) Err(format string, a ...any) {
	l.w.Err(normalizeMsgAndPrefixNss(format, a...))
}

// Crit sends a critical message to the logger.
func (l Logger) Crit(format string, a ...any) {
	l.w.Crit(normalizeMsgAndPrefixNss(format, a...))
}

// normalizeMsgAndPrefixNss prefix msg with NSS before calling NormalizeMsg.
func normalizeMsgAndPrefixNss(format string, a ...any) string {
	format = fmt.Sprintf("nss_aad: %v", format)
	return logger.NormalizeMsg(format, a...)
}
