#include <nss.h>
#include <stdlib.h>

#ifdef INTEGRATIONTESTS

static void __attribute__((constructor))
nsstest_ctor(void)
{
    __nss_configure_lookup("passwd", "files aad");
    __nss_configure_lookup("group", "files aad");
    __nss_configure_lookup("shadow", "files aad");
}

#endif