package pam

/*
#cgo LDFLAGS: -lpam -fPIC
#include <security/pam_ext.h>
#include <syslog.h>
#include <stdlib.h>

void pam_syslog_no_variadic(const pam_handle_t *pamh, int priority, const char *msg) {
	pam_syslog(pamh, priority, "%s", msg);
}

int pam_info_no_variadic(pam_handle_t *pamh, const char *msg) {
	return pam_info(pamh, "%s", msg);
}
*/
import "C"
import (
	"context"
	"fmt"
	"os"
	"unsafe"
)

const (
	ctxKey = "pamhCtxKey"
)

// Handle allows to pass C.pam_handle_t to this package.
type Handle = *C.pam_handle_t

func CtxWithPamh(ctx context.Context, pamh Handle) context.Context {
	return context.WithValue(ctx, ctxKey, pamh)
}

func LogDebug(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_DEBUG, format, a...)
}

func LogInfo(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_INFO, format, a...)
}

func LogWarn(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_WARNING, format, a...)
}

func LogErr(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_ERR, format, a...)
}

func LogCrit(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_CRIT, format, a...)
}

func pamSyslog(ctx context.Context, priority int, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)

	pamh, ok := ctx.Value(ctxKey).(*C.pam_handle_t)
	if !ok {
		prefix := "DEBUG:"
		switch priority {
		case C.LOG_INFO:
			prefix = "INFO:"
		case C.LOG_WARNING:
			prefix = "WARNING:"
		case C.LOG_ERR:
			prefix = "ERROR:"
		case C.LOG_CRIT:
			prefix = "CRITICAL:"
		}
		fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
		return
	}

	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))

	p := C.int(priority)
	C.pam_syslog_no_variadic(pamh, p, cMsg)
}

func Info(ctx context.Context, format string, a ...any) {
	pamh := ctx.Value(ctxKey).(*C.pam_handle_t)

	msg := fmt.Sprintf(format, a...)
	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))

	if errInt := C.pam_info_no_variadic(pamh, cMsg); errInt != C.PAM_SUCCESS {
		LogWarn(ctx, "Failed to display message to user (error %d): %v", errInt, msg)
	}
}
