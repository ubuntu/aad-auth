package testutils

/*
	#include <grp.h>
	#include <pwd.h>
	#include <stdlib.h>
*/
import "C"
import (
	"testing"
	"unsafe"
)

type (
	// CChar allow to cast to a char
	CChar = C.char
	// CSizeT allow to cast to a size_t
	CSizeT = C.size_t
)

// AllowCBuffer returns a new C buffer of buflen. Memory is freed when the test ends.
func AllocCBuffer(t *testing.T, buflen CSizeT) *CChar {
	buf := (*CChar)(C.malloc(C.size_t(buflen)))
	t.Cleanup(func() { C.free(unsafe.Pointer(buf)) })
	return buf
}

/*
 * C representation of passwd helpers, as those can’t be in *_test.go files.
 */

// CPasswd is the struct passwd
type CPasswd = *C.struct_passwd

// NewCPasswd allocates a new C struct passwd.
func NewCPasswd() CPasswd {
	return &C.struct_passwd{}
}

// PubliCPasswd is the public representation to be marshaled and unmashaled on disk.
type PublicCPasswd struct {
	PwName   string `yaml:"pw_name"`
	PwPasswd string `yaml:"pw_passwd"`
	PwUID    uint   `yaml:"pw_uid"`
	PwGID    uint   `yaml:"pw_gid"`
	PwGecos  string `yaml:"pw_gecos"`
	PwDir    string `yaml:"pw_dir"`
	PwShell  string `yaml:"pw_shell"`
}

// ToPublicCPasswd convert the CPasswd struct to a form ready to be converted to yaml.
func (pwd CPasswd) ToPublicCPasswd() PublicCPasswd {
	return PublicCPasswd{
		PwName:   C.GoString(pwd.pw_name),
		PwPasswd: C.GoString(pwd.pw_passwd),
		PwUID:    uint(pwd.pw_uid),
		PwGID:    uint(pwd.pw_gid),
		PwGecos:  C.GoString(pwd.pw_gecos),
		PwDir:    C.GoString(pwd.pw_dir),
		PwShell:  C.GoString(pwd.pw_shell),
	}
}

/*
 * C representation of group helpers, as those can’t be in *_test.go files.
 */

// CGroup is the struct group
type CGroup = *C.struct_group

// NewCGroup allocates a new C struct group.
func NewCGroup() CGroup {
	return &C.struct_group{}
}

// PublicCGroup the public representation to be marshaled and unmashaled on disk.
type PublicCGroup struct {
	GrName   string   `yaml:"gr_name"`
	GrPasswd string   `yaml:"gr_passwd"`
	GrGID    uint     `yaml:"gr_gid"`
	GrMem    []string `yaml:"gr_mem"`
}

// ToPublicCGroup convert the CGroup struct to a form ready to be converted to yaml.
func (g CGroup) ToPublicCGroup(membersNum int) PublicCGroup {
	var members []string
	for _, mem := range unsafe.Slice(g.gr_mem, membersNum) {
		members = append(members, C.GoString(mem))
	}
	return PublicCGroup{
		GrName:   C.GoString(g.gr_name),
		GrPasswd: C.GoString(g.gr_passwd),
		GrGID:    uint(g.gr_gid),
		GrMem:    members,
	}
}
