package main

/*
#cgo LDFLAGS: -fPIC
#include <nss.h>
#include <grp.h>
#include <errno.h>

typedef enum nss_status nss_status;
*/
import "C"
import (
	"context"
	"unsafe"

	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss/group"
	"github.com/ubuntu/aad-auth/internal/user"
)

//export _nss_aad_getgrnam_r
func _nss_aad_getgrnam_r(name *C.char, grp *C.struct_group, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	defer logger.CloseLoggerFromContext(ctx)
	n := C.GoString(name)
	logger.Debug(ctx, "_nss_aad_getgrname_r called for %q", n)
	n = user.NormalizeName(n)

	p, err := group.NewByName(ctx, n)
	if err != nil {
		return errToCStatus(ctx, err, errnop)
	}
	if err = p.ToCgroup(group.CGroup(unsafe.Pointer(grp)), (*group.CChar)(buf), group.CSizeT(buflen)); err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_getgrgid_r
func _nss_aad_getgrgid_r(gid C.gid_t, grp *C.struct_group, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	defer logger.CloseLoggerFromContext(ctx)
	logger.Debug(ctx, "_nss_aad_getgrgid_r called for %d", gid)

	g, err := group.NewByGID(ctx, uint(gid))
	if err != nil {
		return errToCStatus(ctx, err, errnop)
	}
	if err = g.ToCgroup(group.CGroup(unsafe.Pointer(grp)), (*group.CChar)(buf), group.CSizeT(buflen)); err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_setgrent
func _nss_aad_setgrent(stayopen C.int) C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	defer logger.CloseLoggerFromContext(ctx)
	logger.Debug(ctx, "_nss_aad_setgrent called")

	err := group.StartEntryIteration(ctx)
	if err != nil {
		return errToCStatus(ctx, err, nil)
	}

	// Initialization of the database is done in the read primitive
	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_endgrent
func _nss_aad_endgrent() C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	defer logger.CloseLoggerFromContext(ctx)
	logger.Debug(ctx, "_nss_aad_endgrent called")

	err := group.EndEntryIteration(ctx)
	if err != nil {
		return errToCStatus(ctx, err, nil)
	}

	// Closing the database is done in the read primitive
	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_getgrent_r
func _nss_aad_getgrent_r(grbuf *C.struct_group, buf *C.char, buflen C.size_t, errnop *C.int) C.nss_status {
	ctx := ctxWithSyslogLogger(context.Background())
	defer logger.CloseLoggerFromContext(ctx)
	logger.Debug(ctx, "_nss_aad_getgrent_r called")

	g, err := group.NextEntry(ctx)
	if err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	if err = g.ToCgroup(group.CGroup(unsafe.Pointer(grbuf)), (*group.CChar)(buf), group.CSizeT(buflen)); err != nil {
		return errToCStatus(ctx, err, errnop)
	}

	return C.NSS_STATUS_SUCCESS
}
