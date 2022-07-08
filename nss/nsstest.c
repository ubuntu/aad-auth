/*#include <nss.h>
#include <stdlib.h>

static void __attribute__((constructor))
nsstest_ctor(void)
{
    const char *db = getenv("NSSTEST_DB");
        if (db)
            __nss_configure_lookup(db, "aad");
        else
            __nss_configure_lookup("passwd", "aad");

}*/