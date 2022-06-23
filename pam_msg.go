package main

/*
#include <security/pam_ext.h>
#include <syslog.h>
#include <stdlib.h>

void pam_syslog_no_variadic(const pam_handle_t *pamh, int priority, const char *msg) {
	pam_syslog(pamh, priority, "%s", msg);
}
*/
import "C"
import (
	"context"
	"fmt"
	"unsafe"
)

func pamLogDebug(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_DEBUG, format, a...)
}

func pamLogInfo(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_INFO, format, a...)
}

func pamLogWarn(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_WARNING, format, a...)
}

func pamLogErr(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_ERR, format, a...)
}

func pamLogCrit(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_CRIT, format, a...)
}

func pamSyslog(ctx context.Context, priority int, format string, a ...any) {
	pamh := ctx.Value(pamhCtxKey).(*C.pam_handle_t)

	msg := fmt.Sprintf(format, a...)
	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))

	p := C.int(priority)
	C.pam_syslog_no_variadic(pamh, p, cMsg)
}
