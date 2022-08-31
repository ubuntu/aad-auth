#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <nss.h>
#include <pwd.h>
#include <string.h>
#include <sys/types.h>
#include <unistd.h>
#include <errno.h>
#include <ctype.h>

#include <glib.h>
#include <glib/gprintf.h>

#ifndef SCRIPTPATH
#define SCRIPTPATH "/usr/libexec/aad-auth"
#endif

gint run_aad_auth(const char *db, const char *name, const uid_t uid, enum nss_status *nss_exit_status, int *errnop, GPtrArray *entries)
{
    gchar *stdout = NULL;
    gchar *stderr = NULL;
    GError *error = NULL;
    gchar *cmd;

    if (name)
    {
        // Concatenate name with cmd
        cmd = g_strconcat(SCRIPTPATH, " ", "getent", " ", db, " ", name, NULL);
    }
    else if (uid != 0)
    {
        gchar *uid_s = NULL;
        uid_s = g_strdup_printf(" %u", uid);
        cmd = g_strconcat(SCRIPTPATH, " ", "getent", " ", db, " ", uid_s, NULL);
        g_free(uid_s);
    }
    else
    {
        cmd = g_strconcat(SCRIPTPATH, " ", "getent", " ", db, NULL);
    }

    gint exit_status;
    if (!g_spawn_command_line_sync(cmd, &stdout, &stderr, &exit_status, &error) || exit_status != 0)
    {
        *errnop = ENOENT;
        g_free(cmd);
        return exit_status;
    }
    g_free(cmd);

    gchar **lines = g_strsplit(stdout, "\n", -1);
    for (gint i = 0; lines[i]; i++)
    {
        // first line is nss_exit_status:errno
        if (i == 0)
        {
            gchar **statuses = g_strsplit(lines[i], ":", 2);
            *nss_exit_status = atoi(statuses[0]);
            *errnop = atoi(statuses[1]);
            g_strfreev(statuses);
            continue;
        }

        gchar *v = g_strdup(lines[i]);
        g_ptr_array_add(entries, (gpointer)v);
    }
    g_strfreev(lines);

    return 0;
}

enum nss_status fetch_info(const char *db, const char *name, const uid_t uid, GPtrArray *all_entries, guint *all_entries_index, gchar **entry, int *errnop)
{
    gint nss_exit_status = NSS_STATUS_SUCCESS;

    if (name != NULL || uid != 0)
    {
        GPtrArray *entries = g_ptr_array_new();
        gint exit_status = run_aad_auth(db, name, uid, &nss_exit_status, errnop, entries);
        if (exit_status != 0)
        {
            *errnop = ENOENT;
            return NSS_STATUS_UNAVAIL;
        }
        *entry = g_strdup((gchar *)g_ptr_array_index(entries, 0));
        g_ptr_array_free(entries, TRUE);
    }
    else if (all_entries->len == 0)
    {
        gint exit_status = run_aad_auth(db, name, uid, &nss_exit_status, errnop, all_entries);
        if (exit_status != 0)
        {
            *errnop = ENOENT;
            return NSS_STATUS_UNAVAIL;
        }
        *entry = g_strdup((gchar *)g_ptr_array_index(all_entries, *all_entries_index));
        (*all_entries_index)++;
    }
    else if (*all_entries_index < all_entries->len)
    {
        *entry = g_strdup((gchar *)g_ptr_array_index(all_entries, *all_entries_index));
        (*all_entries_index)++;
    }
    else
    {
        // iteration has ended, return our own status
        (*all_entries_index) = 0;
        *errnop = ENOENT;
        return NSS_STATUS_UNAVAIL;
    }

    return nss_exit_status;
}
