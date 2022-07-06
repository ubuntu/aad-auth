package pam

/*
#cgo LDFLAGS: -lpam -fPIC
#include <security/pam_ext.h>
#include <syslog.h>
#include <stdlib.h>

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
	ctxPamhKey = "pamhCtxKey"
)

func CtxWithPamh(ctx context.Context, pamh Handle) context.Context {
	return context.WithValue(ctx, ctxPamhKey, pamh)
}

func Info(ctx context.Context, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)

	pamh, ok := ctx.Value(ctxPamhKey).(*C.pam_handle_t)
	if !ok {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to display message to user (no pam attached): %v", msg)
		pamSyslog(pamh, C.LOG_WARNING, "Failed to display message to user (no pam attached): %v", msg)
	}

	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))

	if errInt := C.pam_info_no_variadic(pamh, cMsg); errInt != C.PAM_SUCCESS {
		pamSyslog(pamh, C.LOG_WARNING, "Failed to display message to user (error %d): %v", errInt, msg)
	}
}
