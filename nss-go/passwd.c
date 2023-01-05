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

#include "common.h"

GPtrArray *all_pwd_entries = NULL;
guint all_pwd_entries_index = 0;

enum nss_status pwd_search(const char *name, const uid_t uid, struct passwd *pw, int *errnop, char *buffer, size_t buflen)
{
	UNUSED(buffer);
	UNUSED(buflen);

	if (all_pwd_entries == NULL)
	{
		all_pwd_entries = g_ptr_array_new();
	}

	gchar *entry = calloc(1000, sizeof(gchar));
	gint exit_status = fetch_info("passwd", name, uid, all_pwd_entries, &all_pwd_entries_index, &entry, errnop);

	// return the exit status if no success
	if (exit_status != NSS_STATUS_SUCCESS)
	{
		g_free(entry);
		return exit_status;
	}

	gchar **tokens = g_strsplit(entry, ":", 7);
	g_free(entry);

	pw->pw_name = g_strdup(tokens[0]);
	pw->pw_passwd = g_strdup(tokens[1]);
	pw->pw_uid = strtol(tokens[2], NULL, 10);
	pw->pw_gid = strtol(tokens[3], NULL, 10);
	pw->pw_gecos = g_strdup(tokens[4]);
	pw->pw_dir = g_strdup(tokens[5]);
	pw->pw_shell = g_strdup(tokens[6]);
	g_strfreev(tokens);

	return exit_status;
}

enum nss_status _nss_aad_getpwuid_r(uid_t uid, struct passwd *result, char *buf, size_t buflen, int *errnop)
{
	*errnop = 0;
	if (result)
		return pwd_search(NULL, uid, result, errnop, buf, buflen);
	else
		return NSS_STATUS_UNAVAIL;
}

enum nss_status _nss_aad_getpwnam_r(const char *name, struct passwd *result, char *buf, size_t buflen, int *errnop)
{
	*errnop = 0;
	if (result)
		return pwd_search(name, 0, result, errnop, buf, buflen);
	else
		return NSS_STATUS_UNAVAIL;
}

enum nss_status _nss_aad_setpwent(void)
{
	if (all_pwd_entries != NULL)
	{
		g_ptr_array_free(all_pwd_entries, TRUE);
		all_pwd_entries = NULL;
	}

	all_pwd_entries_index = 0;
	return NSS_STATUS_SUCCESS;
}

enum nss_status _nss_aad_endpwent(void)
{
	if (all_pwd_entries != NULL)
	{
		g_ptr_array_free(all_pwd_entries, TRUE);
		all_pwd_entries = NULL;
	}

	all_pwd_entries_index = 0;
	return NSS_STATUS_SUCCESS;
}

enum nss_status _nss_aad_getpwent_r(struct passwd *result, char *buf, size_t buflen, int *errnop)
{
	*errnop = -1;

	if (result)
		return pwd_search(NULL, 0, result, errnop, buf, buflen);

	return NSS_STATUS_UNAVAIL;
}
