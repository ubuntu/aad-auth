package nss

import (
	"fmt"
	"log/syslog"
)

// Logger is the logger connected to syslog.
type Logger struct {
	w *syslog.Writer
}

// NewLogger returns a logger ready to log to syslog.
func NewLogger(priority syslog.Priority) (l *Logger, err error) {
	w, err := syslog.New(priority, "")
	if err != nil {
		return nil, fmt.Errorf("can't create nss logger: %v", err)
	}

	return &Logger{
		w: w,
	}, nil
}

// Debug sends a debug level message to the logger.
func (l Logger) Debug(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	l.w.Debug(msg)
}

// Info sends an informational message to the logger.
func (l Logger) Info(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	l.w.Info(msg)
}

// Warn sends a warning level message to the logger.
func (l Logger) Warn(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	l.w.Warning(msg)
}

// Err sends an error level message to the logger.
func (l Logger) Err(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	l.w.Err(msg)
}

// Crit sends a critical message to the logger.
func (l Logger) Crit(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	l.w.Crit(msg)
}
