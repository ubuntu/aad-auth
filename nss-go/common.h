#include <nss.h>
#include <glib.h>

#define UNUSED(x) (void)(x)

enum nss_status fetch_info(const char *db, const char *name, const uid_t uid, GPtrArray *all_entries, guint *all_entries_index, gchar **entry, int *errnop);
