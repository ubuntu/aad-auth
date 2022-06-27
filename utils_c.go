package main

/*
#include <stdlib.h>
#include <string.h>

char *string_from_argv(int i, char **argv) {
  return strdup(argv[i]);
}
*/
import "C"
import (
	"unsafe"
)

func sliceFromArgv(argc C.int, argv **C.char) []string {
	r := make([]string, 0, argc)
	for i := 0; i < int(argc); i++ {
		s := C.string_from_argv(C.int(i), argv)
		defer C.free(unsafe.Pointer(s))
		r = append(r, C.GoString(s))
	}
	return r
}
