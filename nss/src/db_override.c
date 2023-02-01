#include "nss.h"

#ifdef INTEGRATION_TESTS
// db_override configures the local nss lookup to use the aad database.
void db_override() {
    __nss_configure_lookup("passwd", "files aad");
    __nss_configure_lookup("group", "files aad");
    __nss_configure_lookup("shadow", "files aad");
}
#endif