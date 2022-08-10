package pam

/*
#cgo LDFLAGS: -lpam -fPIC
#include <security/pam_ext.h>
#include <syslog.h>
#include <stdlib.h>

void pam_syslog_no_variadic(const pam_handle_t *pamh, int priority, const char *msg) {
	pam_syslog(pamh, priority, "%s", msg);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// Priority is the level of the message
type Priority int

const (
	// LogInfo matches the syslog Info level
	LogInfo Priority = 6
	// LogDebug matches the syslog Debug level
	LogDebug Priority = 7

	debugWelcome = "aad auth debug enabled\n"
)

// Handle allows to pass C.pam_handle_t to this package.
type Handle = *C.pam_handle_t

// Logger is the logger connected to pam infra.
type Logger struct {
	pamHandle Handle
	priority  Priority

	logWithPam func(pamh Handle, priority int, format string, a ...any)
}

// optionsLogger are the options supported by Logger.
type optionsLogger struct {
	logWithPam func(pamh Handle, priority int, format string, a ...any)
}

// OptionLogger represents one functional option passed to Logger.
type OptionLogger func(*optionsLogger)

// NewLogger returns a Logger hanging the Logger information.
func NewLogger(pamHandle Handle, priority Priority, opts ...OptionLogger) Logger {
	o := optionsLogger{
		logWithPam: pamSyslog,
	}
	// applied options
	for _, opt := range opts {
		opt(&o)
	}

	l := Logger{
		pamHandle:  pamHandle,
		priority:   priority,
		logWithPam: o.logWithPam,
	}
	l.Debug("aad auth debug enabled\n")
	return l
}

// Debug sends a debug level message to the logger.
func (l Logger) Debug(format string, a ...any) {
	if l.priority < LogDebug {
		return
	}
	l.logWithPam(l.pamHandle, C.LOG_DEBUG, format, a...)
}

// Info sends an informational message to the logger.
func (l Logger) Info(format string, a ...any) {
	l.logWithPam(l.pamHandle, C.LOG_INFO, format, a...)
}

// Warn sends a warning level message to the logger.
func (l Logger) Warn(format string, a ...any) {
	l.logWithPam(l.pamHandle, C.LOG_WARNING, format, a...)
}

// Err sends an error level message to the logger.
func (l Logger) Err(format string, a ...any) {
	l.logWithPam(l.pamHandle, C.LOG_ERR, format, a...)
}

// Crit sends a critical message to the logger.
func (l Logger) Crit(format string, a ...any) {
	l.logWithPam(l.pamHandle, C.LOG_CRIT, format, a...)
}

// Close does nothing for PAM
func (l Logger) Close() error {
	return nil
}

// pamSyslog sends directly logs syslog via PAM.
func pamSyslog(pamh Handle, priority int, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)

	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))

	p := C.int(priority)
	C.pam_syslog_no_variadic(pamh, p, cMsg)
}
