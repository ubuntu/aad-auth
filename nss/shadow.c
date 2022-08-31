#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <nss.h>
#include <shadow.h>
#include <string.h>
#include <sys/types.h>
#include <unistd.h>
#include <errno.h>
#include <ctype.h>

#include <glib.h>

#include "common.h"

GPtrArray *all_spwd_entries = NULL;
guint all_spwd_entries_index = 0;

enum nss_status spwd_search(const char *name, struct spwd *spw, int *errnop, char *buffer, size_t buflen)
{
	UNUSED(buffer);
	UNUSED(buflen);

	if (all_spwd_entries == NULL)
	{
		all_spwd_entries = g_ptr_array_new();
	}

	gchar *entry = calloc(1000, sizeof(gchar));
	gint exit_status = fetch_info("shadow", name, 0, all_spwd_entries, &all_spwd_entries_index, &entry, errnop);

	// return the exit status if no success
	if (exit_status != NSS_STATUS_SUCCESS)
	{
		g_free(entry);
		return exit_status;
	}

	gchar **tokens = g_strsplit(entry, ":", 9);
	g_free(entry);

	spw->sp_namp = g_strdup(tokens[0]);
	spw->sp_pwdp = g_strdup(tokens[1]);
	spw->sp_lstchg = strtol(tokens[2], NULL, 10);
	spw->sp_min = strtol(tokens[3], NULL, 10);
	spw->sp_max = strtol(tokens[4], NULL, 10);
	spw->sp_warn = strtol(tokens[5], NULL, 10);
	spw->sp_inact = strtol(tokens[6], NULL, 10);
	spw->sp_expire = strtol(tokens[7], NULL, 10);
	spw->sp_flag = strtoul(tokens[8], NULL, 10);

	g_strfreev(tokens);

	return exit_status;
}

enum nss_status _nss_aad_getspnam_r(const char *name, struct spwd *result, char *buf, size_t buflen, int *errnop)
{
	*errnop = 0;
	if (result)
		return spwd_search(name, result, errnop, buf, buflen);
	else
		return NSS_STATUS_UNAVAIL;
}

enum nss_status _nss_aad_setspent(void)
{
	if (all_spwd_entries != NULL)
	{
		g_ptr_array_free(all_spwd_entries, TRUE);
		all_spwd_entries = NULL;
	}

	all_spwd_entries_index = 0;
	return NSS_STATUS_SUCCESS;
}

enum nss_status _nss_aad_endspent(void)
{
	if (all_spwd_entries != NULL)
	{
		g_ptr_array_free(all_spwd_entries, TRUE);
		all_spwd_entries = NULL;
	}

	all_spwd_entries_index = 0;
	return NSS_STATUS_SUCCESS;
}

enum nss_status _nss_aad_getspent_r(struct spwd *result, char *buf, size_t buflen, int *errnop)
{
	*errnop = -1;

	if (result)
		return spwd_search(NULL, result, errnop, buf, buflen);

	return NSS_STATUS_UNAVAIL;
}
