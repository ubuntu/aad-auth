use libnss::interop::Response;
use libnss::shadow::{Shadow, ShadowHooks};

use crate::{
    cache::{CacheError, Shadow as CacheShadow},
    debug,
};

pub struct AADShadow;

impl ShadowHooks for AADShadow {
    /// get_all_entries retrieves all the shadow entries from the cache database.
    fn get_all_entries() -> Response<Vec<Shadow>> {
        debug!("get_all_entries for shadow");

        let c = match super::new_cache() {
            Ok(c) => c,
            Err(err) => return super::cache_result_to_nss_status(Err(err)),
        };

        let r = result_vec_cache_shadow_to_result_vec_nss_shadow(c.get_all_shadows());
        super::cache_result_to_nss_status(r)
    }

    /// get_entry_by_name retrieves a shadow entry by user name.
    fn get_entry_by_name(name: String) -> Response<Shadow> {
        debug!("get_entry_by_name for shadow for name: {name}");

        let c = match super::new_cache() {
            Ok(c) => c,
            Err(err) => return super::cache_result_to_nss_status(Err(err)),
        };

        let mut r = result_cache_shadow_to_result_nss_shadow(c.get_shadow_by_name(&name));

        // we want to prevent pam_unix using this field to use a cached account without calling pam_aad.
        if let Ok(entry) = &mut r {
            entry.passwd = "*".to_string();
        }

        super::cache_result_to_nss_status(r)
    }
}

/// cache_shadow_to_nss_shadow converts a `Cache::shadow` object to a `libnss::shadow` object.
fn cache_shadow_to_nss_shadow(entry: CacheShadow) -> Shadow {
    debug!("found record: {:?}", entry);

    Shadow {
        name: entry.name,
        // we want to prevent pam_unix using this field to use a cached account without calling pam_aad.
        passwd: entry.passwd,
        last_change: entry.last_pwd_change,
        change_min_days: entry.min_pwd_age,
        change_max_days: entry.max_pwd_age,
        change_warn_days: entry.pwd_warn_period,
        change_inactive_days: entry.pwd_inactivity,
        expire_date: entry.expiration_date,
        reserved: usize::MAX,
    }
}

/// result_cache_shadow_to_result_nss_shadow matches errors code between the cache object and NSS.
fn result_cache_shadow_to_result_nss_shadow(
    entry: Result<CacheShadow, CacheError>,
) -> Result<Shadow, CacheError> {
    Ok(cache_shadow_to_nss_shadow(entry?))
}

/// result_vec_cache_shadow_to_result_vec_nss_shadow converts a list of `CacheDB::Shadow` entries
/// to a list `libnss::Shadow` entries.
fn result_vec_cache_shadow_to_result_vec_nss_shadow(
    entry: Result<Vec<CacheShadow>, CacheError>,
) -> Result<Vec<Shadow>, CacheError> {
    let mut v = Vec::new();
    for e in entry? {
        v.push(cache_shadow_to_nss_shadow(e));
    }

    Ok(v)
}

#[cfg(test)]
mod mod_tests;
