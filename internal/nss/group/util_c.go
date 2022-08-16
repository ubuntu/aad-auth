package group

/*
#include <nss.h>
#include <grp.h>
*/
import "C"
import (
	"bytes"
	"unsafe"

	"github.com/ubuntu/aad-auth/internal/nss"
)

type (
	// CGroup is the struct group
	CGroup = *C.struct_group
	// CChar allows to cast to a char
	CChar = C.char
	// CSizeT allows to cast to a size_t
	CSizeT = C.size_t
)

// ToCgroup transforms the Go Group struct to a C struct group.
// The function will check first for errors to transform them to corresponding nss status.
//
// We are casting an array of char* (addresses) in a char* buffer. Forces the checker to say we know
// what we do and don’t error on it.
//
//go:nocheckptr
func (g Group) ToCgroup(grp CGroup, buf *CChar, buflen CSizeT) error {
	// Ensure the buffer is big enough for all fields of group, with an offset.
	// Calculate the size of members array.
	sizeOfPChar := unsafe.Sizeof(uintptr(0))
	lenMembers := int(sizeOfPChar) * (len(g.members) + 1) // add pointers array table
	// Add each member size with finale \0
	for _, m := range g.members {
		lenMembers += len(m) + 1
	}
	// 2 is the number of fields of type char * in the structure 'group'
	if int(buflen) < len(g.name)+len(g.passwd)+lenMembers+2 {
		return nss.ErrTryAgainERange
	}

	// Transform the C buffer to a Go one.
	// TODO: test replacing with C.GoBytes…
	gobuf := (*[1 << 30]byte)(unsafe.Pointer(buf))[:buflen:buflen]
	b := bytes.NewBuffer(gobuf)
	b.Reset()

	// Points the C groups struct field to the current address of the buffer (start of current field value),
	// then file the buffer with the value we want to use.
	grp.gr_name = (*C.char)(unsafe.Pointer(&gobuf[b.Len()]))
	b.WriteString(g.name)
	b.WriteByte(0)

	grp.gr_passwd = (*C.char)(unsafe.Pointer(&gobuf[b.Len()]))
	b.WriteString(g.passwd)
	b.WriteByte(0)

	var membersAddresses []*C.char
	// Write members data
	for _, s := range g.members {
		membersAddresses = append(membersAddresses, (*C.char)(unsafe.Pointer(&gobuf[b.Len()])))
		b.WriteString(s)
		b.WriteByte(0)
	}
	// Write members addresses
	bufp := (**C.char)(unsafe.Pointer(&gobuf[b.Len()]))
	grp.gr_mem = bufp
	b.Write(make([]byte, int(sizeOfPChar)*(len(membersAddresses))))
	for _, addr := range membersAddresses {
		*bufp = addr
		/*pp := C.GoBytes(unsafe.Pointer(uintptr(unsafe.Pointer(bufp))), C.int(sizeOfPChar))
		b.Write(pp)*/
		bufp = (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(bufp)) + sizeOfPChar))
	}
	b.Write(make([]byte, int(sizeOfPChar))) // nil array termination.

	// gid are not pointers, but just the uint itself.
	grp.gr_gid = C.uint(g.gid)

	return nil
}
