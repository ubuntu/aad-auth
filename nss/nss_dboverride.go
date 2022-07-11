package main

// TODO: move it to subdirectory for tests
// ./libnss_aad.so.2

/*
#include <nss.h>
#include <stdlib.h>

static void __attribute__((constructor))
nsstest_ctor(void)
{
    __nss_configure_lookup("passwd", "files aad");
    __nss_configure_lookup("group", "files aad");
    __nss_configure_lookup("shadow", "files aad");

}
*/
import "C"
