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
	"errors"
	"fmt"
	"unsafe"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/group"
	"github.com/ubuntu/aad-auth/internal/nss"
)

//export _nss_aad_getgrnam_r
func _nss_aad_getgrnam_r(name *C.char, grp *C.struct_group, buf *C.char, buflen C.size_t, result **C.struct_group) C.nss_status {
	n := C.GoString(name)
	p, err := group.NewByName(n)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}
	if err = p.ToCgroup(
		group.CGroup(unsafe.Pointer(grp)),
		(*group.CChar)(buf),
		group.CSizeT(buflen),
		(*group.CGroup)(unsafe.Pointer(result))); err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}

	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_getgrgid_r
func _nss_aad_getgrgid_r(gid C.gid_t, grp *C.struct_group, buf *C.char, buflen C.size_t, result **C.struct_group) C.nss_status {
	g, err := group.NewByGID(uint(gid))
	if err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}
	if err = g.ToCgroup(
		group.CGroup(unsafe.Pointer(grp)),
		(*group.CChar)(buf),
		group.CSizeT(buflen),
		(*group.CGroup)(unsafe.Pointer(result))); err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}

	return C.NSS_STATUS_SUCCESS
}

//export _nss_aad_setgrent
func _nss_aad_setgrent() {
	// Initialization of the database is done in the read primitive
}

//export _nss_aad_endgrent
func _nss_aad_endgrent() {
	// Closing the database is done in the read primitive
}

//export _nss_aad_getgrent_r
func _nss_aad_getgrent_r(grbuf *C.struct_group, buf *C.char, buflen C.size_t, grbufp **C.struct_group) C.nss_status {
	g, err := group.NextEntry()
	if errors.Is(err, cache.ErrNoEnt) {
		return C.ENOENT
	}
	if err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}

	if err = g.ToCgroup(
		group.CGroup(unsafe.Pointer(grbuf)),
		(*group.CChar)(buf),
		group.CSizeT(buflen),
		(*group.CGroup)(unsafe.Pointer(grbufp))); err != nil {
		fmt.Printf("ERROR: %v\n", err) // TODO: log
		return (C.nss_status)(nss.ErrToCStatus(err))
	}

	return C.NSS_STATUS_SUCCESS
}
