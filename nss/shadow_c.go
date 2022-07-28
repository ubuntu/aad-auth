package main

/*
#cgo LDFLAGS: -fPIC
#include <nss.h>
#include <shadow.h>
#include <errno.h>

typedef enum nss_status nss_status;
*/
import "C"
import (
	"context"
	"unsafe"

	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
	"github.com/ubuntu/aad-auth/internal/nss/shadow"
	"github.com/ubuntu/aad-auth/internal/user"
)

//export _nss_aad_getspnam_r
func _nss_aad_getspnam_r(name *C.char, spwd *C.struct_spwd, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	ctx := nss.CtxWithSyslogLogger(context.Background())
	defer logger.CloseLoggerFromContext(ctx)
	n := C.GoString(name)
	logger.Debug(ctx, "_nss_aad_getspnam_r called for %q", n)
	n = user.NormalizeName(n)

	sp, err := shadow.NewByName(ctx, n)
	if err != nil {
		return errToCStatus(ctx, err, errnop)
	}
	if err = sp.ToCshadow(shadow.CShadow(unsafe.Pointer(spwd)), (*shadow.CChar)(buf), shadow.CSizeT(buflen)); err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_setspent
func _nss_aad_setspent() C.nss_status {
	ctx := nss.CtxWithSyslogLogger(context.Background())
	defer logger.CloseLoggerFromContext(ctx)
	logger.Debug(ctx, "_nss_aad_setspent called")

	err := shadow.StartEntryIteration(ctx)
	if err != nil {
		return errToCStatus(ctx, err, nil)
	}

	// Initialization of the database is done in the read primitive
	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_endspent
func _nss_aad_endspent() C.nss_status {
	ctx := nss.CtxWithSyslogLogger(context.Background())
	defer logger.CloseLoggerFromContext(ctx)
	logger.Debug(ctx, "_nss_aad_endspent called")

	err := shadow.EndEntryIteration(ctx)
	if err != nil {
		return errToCStatus(ctx, err, nil)
	}

	// Closing the database is done in the read primitive
	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_getspent_r
func _nss_aad_getspent_r(spwd *C.struct_spwd, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	ctx := nss.CtxWithSyslogLogger(context.Background())
	defer logger.CloseLoggerFromContext(ctx)
	logger.Debug(ctx, "_nss_aad_getspent_r called")

	sp, err := shadow.NextEntry(ctx)
	if err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	if err = sp.ToCshadow(shadow.CShadow(unsafe.Pointer(spwd)), (*shadow.CChar)(buf), shadow.CSizeT(buflen)); err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}
