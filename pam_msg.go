package main

/*
#include <security/pam_ext.h>
#include <syslog.h>
#include <stdlib.h>

void pam_syslog_no_variadic(const pam_handle_t *pamh, int priority, const char *fmt) {
	pam_syslog(pamh, priority, "%s", fmt);
}
*/
import "C"
import (
	"context"
	"fmt"
	"unsafe"
)

func pamDebug(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_DEBUG, format, a...)
}

func pamInfo(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_INFO, format, a...)
}

func pamWarn(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_WARNING, format, a...)
}

func pamErr(ctx context.Context, format string, a ...any) {
	pamSyslog(ctx, C.LOG_ERR, format, a...)
}

func pamCrit(ctx context.Context, format string, a ...any) {
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
