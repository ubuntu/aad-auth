package shadow

/*
#include <nss.h>
#include <shadow.h>
*/
import "C"
import (
	"bytes"
	"unsafe"

	"github.com/ubuntu/aad-auth/internal/nss"
)

type (
	// CShadow is the struct passwd
	CShadow = *C.struct_spwd
	// CChar allows to cast to a char
	CChar = C.char
	// CSizeT allows to cast to a size_t
	CSizeT = C.size_t
)

// ToCshadow transforms the Go shadow struct to a C struct shadow, filling buffer, result and nss_status.
// The function will check first for errors to transform them to corresponding nss status.
func (s Shadow) ToCshadow(spwd CShadow, buf *CChar, buflen CSizeT) error {
	// Ensure the buffer is big enough for all fields of passwd, with an offset.
	// 2 is the number of fields of type char * in the structure 'shadow'
	if int(buflen) < len(s.name)+len(s.passwd)+2 {
		return nss.ErrTryAgainERange
	}

	// Transform the C buffer to a Go one.
	gobuf := (*[1 << 30]byte)(unsafe.Pointer(buf))[:buflen:buflen]
	b := bytes.NewBuffer(gobuf)
	b.Reset()

	// Points the C passwd struct field to the current address of the buffer (start of current field value),
	// then file the buffer with the value we want to use.
	spwd.sp_namp = (*C.char)(unsafe.Pointer(&gobuf[b.Len()]))
	b.WriteString(s.name)
	b.WriteByte(0)

	spwd.sp_pwdp = (*C.char)(unsafe.Pointer(&gobuf[b.Len()]))
	b.WriteString(s.passwd)
	b.WriteByte(0)

	// those are not pointers, but just the int itself.
	spwd.sp_lstchg = C.long(s.lstchg)
	spwd.sp_min = C.long(s.min)
	spwd.sp_max = C.long(s.max)
	spwd.sp_warn = C.long(s.warn)
	spwd.sp_inact = C.long(s.inact)
	spwd.sp_expire = C.long(s.expire)
	spwd.sp_flag = C.ulong(^uint(0))

	return nil
}
