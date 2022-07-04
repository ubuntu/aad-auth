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

// Handle allows to pass C.pam_handle_t to this package.
type Handle = *C.pam_handle_t

// Logger is the logger connected to pam infra.
type Logger struct {
	pamHandle Handle
}

// NewLogger returns a Logger hanging the Logger information.
func NewLogger(pamHandle Handle) Logger {
	return Logger{
		pamHandle: pamHandle,
	}
}

// Debug sends a debug level message to the logger.
func (l Logger) Debug(format string, a ...any) {
	pamSyslog(l.pamHandle, C.LOG_DEBUG, format, a...)
}

// Info sends an informational message to the logger.
func (l Logger) Info(format string, a ...any) {
	pamSyslog(l.pamHandle, C.LOG_INFO, format, a...)
}

// Warn sends a warning level message to the logger.
func (l Logger) Warn(format string, a ...any) {
	pamSyslog(l.pamHandle, C.LOG_WARNING, format, a...)
}

// Err sends an error level message to the logger.
func (l Logger) Err(format string, a ...any) {
	pamSyslog(l.pamHandle, C.LOG_ERR, format, a...)
}

// Crit sends a critical message to the logger.
func (l Logger) Crit(format string, a ...any) {
	pamSyslog(l.pamHandle, C.LOG_CRIT, format, a...)
}

func pamSyslog(pamh Handle, priority int, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)

	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))

	p := C.int(priority)
	C.pam_syslog_no_variadic(pamh, p, cMsg)
}
