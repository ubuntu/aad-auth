use libc::gid_t;

use libnss::group::{Group, GroupHooks};
use libnss::interop::Response;

use crate::{
    cache::{CacheError, Group as CacheGroup},
    debug,
};

pub struct AADGroup;

impl GroupHooks for AADGroup {
    /// get_all_entries retrieves all group entries from the cache database.
    fn get_all_entries() -> Response<Vec<Group>> {
        debug!("get_all_entries for group");

        let c = match super::new_cache() {
            Ok(c) => c,
            Err(e) => return super::cache_result_to_nss_status(Err(e)),
        };

        let groups = result_vec_cache_group_to_result_vec_nss_group(c.get_all_groups());
        super::cache_result_to_nss_status(groups)
    }

    /// get_entry_by_gid retrieves a group entry by gid.
    fn get_entry_by_gid(gid: gid_t) -> Response<Group> {
        debug!("get_entry_by_gid for group with gid: {gid}");

        let c = match super::new_cache() {
            Ok(c) => c,
            Err(e) => return super::cache_result_to_nss_status(Err(e)),
        };

        let r = result_cache_group_to_result_nss_group(c.get_group_by_gid(gid));
        super::cache_result_to_nss_status(r)
    }

    /// get_entry_by_name retrieves a group entry by name.
    fn get_entry_by_name(name: String) -> Response<Group> {
        debug!("get_entry_by_gid for group with name: {name}");

        let c = match super::new_cache() {
            Ok(c) => c,
            Err(e) => return super::cache_result_to_nss_status(Err(e)),
        };

        let r = result_cache_group_to_result_nss_group(c.get_group_by_name(&name));
        super::cache_result_to_nss_status(r)
    }
}

/// cache_group_to_nss_group converts a `cache::Group` object to a `libnss::Group` object.
fn cache_group_to_nss_group(entry: CacheGroup) -> Group {
    debug!("found record: {:?}", entry);

    Group {
        name: entry.name,
        passwd: entry.passwd,
        gid: entry.gid,
        members: entry.members,
    }
}

/// result_cache_group_to_result_nss_group converts a `Result<cache::Group>` to a `Result<libnss::Group>`.
fn result_cache_group_to_result_nss_group(
    entry: Result<CacheGroup, CacheError>,
) -> Result<Group, CacheError> {
    Ok(cache_group_to_nss_group(entry?))
}

/// result_vec_cache_group_to_result_vec_nss_group converts a `Result<Vec<cache::Group>>` to
/// a `Result<Vec<libnss::Group>>`.
fn result_vec_cache_group_to_result_vec_nss_group(
    entry: Result<Vec<CacheGroup>, CacheError>,
) -> Result<Vec<Group>, CacheError> {
    let mut groups = Vec::new();
    for group in entry? {
        groups.push(cache_group_to_nss_group(group));
    }

    Ok(groups)
}

#[cfg(test)]
mod mod_tests;
