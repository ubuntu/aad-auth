#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <nss.h>
#include <grp.h>
#include <string.h>
#include <sys/types.h>
#include <unistd.h>
#include <errno.h>
#include <ctype.h>

#include <glib.h>
#include <glib/gprintf.h>

#include "common.h"

GPtrArray *all_grp_entries = NULL;
guint all_grp_entries_index = 0;

enum nss_status grp_search(const char *name, const gid_t gid, struct group *gr, int *errnop, char *buffer, size_t buflen)
{
	UNUSED(buffer);
	UNUSED(buflen);

	if (all_grp_entries == NULL)
	{
		all_grp_entries = g_ptr_array_new();
	}

	gchar *entry = calloc(1000, sizeof(gchar));
	gint exit_status = fetch_info("group", name, gid, all_grp_entries, &all_grp_entries_index, &entry, errnop);

	// return the exit status if no success
	if (exit_status != NSS_STATUS_SUCCESS)
	{
		g_free(entry);
		return exit_status;
	}

	gchar **tokens = g_strsplit(entry, ":", 4);
	g_free(entry);

	gr->gr_name = g_strdup(tokens[0]);
	gr->gr_passwd = g_strdup(tokens[1]);
	gr->gr_gid = strtol(tokens[2], NULL, 10);
	gr->gr_mem = g_strsplit(tokens[3], ",", -1);

	g_strfreev(tokens);

	return exit_status;
}

enum nss_status _nss_aad_getgrgid_r(gid_t gid, struct group *result, char *buf, size_t buflen, int *errnop)
{
	*errnop = 0;
	if (result)
		return grp_search(NULL, gid, result, errnop, buf, buflen);
	else
		return NSS_STATUS_UNAVAIL;
}

enum nss_status _nss_aad_getgrnam_r(const char *name, struct group *result, char *buf, size_t buflen, int *errnop)
{
	*errnop = 0;
	if (result)
		return grp_search(name, 0, result, errnop, buf, buflen);
	else
		return NSS_STATUS_UNAVAIL;
}

enum nss_status _nss_aad_setgrent(void)
{
	if (all_grp_entries != NULL)
	{
		g_ptr_array_free(all_grp_entries, TRUE);
		all_grp_entries = NULL;
	}

	all_grp_entries_index = 0;
	return NSS_STATUS_SUCCESS;
}

enum nss_status _nss_aad_endgrent(void)
{
	if (all_grp_entries != NULL)
	{
		g_ptr_array_free(all_grp_entries, TRUE);
		all_grp_entries = NULL;
	}

	all_grp_entries_index = 0;
	return NSS_STATUS_SUCCESS;
}

enum nss_status _nss_aad_getgrent_r(struct group *result, char *buf, size_t buflen, int *errnop)
{
	*errnop = -1;

	if (result)
		return grp_search(NULL, 0, result, errnop, buf, buflen);

	return NSS_STATUS_UNAVAIL;
}
