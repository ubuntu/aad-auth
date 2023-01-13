use libc::uid_t;

use libnss::interop::Response;
use libnss::passwd::{Passwd, PasswdHooks};

use crate::cache::{CacheError, Passwd as CachePasswd};
use crate::{cache_result_to_nss_status, debug, new_cache, LOGPREFIX};

pub struct AADPasswd;

impl PasswdHooks for AADPasswd {
    // get_all_entries retrieves all the password entries from the cache database.
    fn get_all_entries() -> Response<Vec<Passwd>> {
        debug!("get_all_entries for passwd");

        let c = match new_cache() {
            Ok(c) => c,
            Err(e) => return cache_result_to_nss_status(Err(e)),
        };

        let r = result_vec_cache_passwd_to_result_vec_nss_passwd(c.get_all_passwd());
        cache_result_to_nss_status(r)
    }

    // get_entry_by_uid retrieves a password entry by user id.
    fn get_entry_by_uid(uid: uid_t) -> Response<Passwd> {
        debug!("get_entry_by_uid for passwd for uid: {}", uid);

        let c = match new_cache() {
            Ok(c) => c,
            Err(e) => return cache_result_to_nss_status(Err(e)),
        };

        let r = result_cache_passwd_to_result_nss_passwd(c.get_passwd_from_uid(uid));
        cache_result_to_nss_status(r)
    }

    // get_entry_by_name retrieves a password entry by user name.
    fn get_entry_by_name(name: String) -> Response<Passwd> {
        debug!("get_entry_by_name for passwd for name: {}", name);

        let c = match new_cache() {
            Ok(c) => c,
            Err(e) => return cache_result_to_nss_status(Err(e)),
        };

        let r = result_cache_passwd_to_result_nss_passwd(c.get_passwd_from_name(&name));
        cache_result_to_nss_status(r)
    }
}

// cache_passwd_to_nss_passwd matches a cache password object to an NSS type.
fn cache_passwd_to_nss_passwd(entry: CachePasswd) -> Passwd {
    debug!("found record: {:?}", entry);

    Passwd {
        name: entry.name,
        passwd: entry.passwd,
        uid: entry.uid,
        gid: entry.gid,
        gecos: entry.gecos,
        dir: entry.home,
        shell: entry.shell,
    }
}

// result_cache_passwd_to_result_nss_passwd matches errors code between the cache object and NSS.
fn result_cache_passwd_to_result_nss_passwd(
    entry: Result<CachePasswd, CacheError>,
) -> Result<Passwd, CacheError> {
    match entry {
        Err(e) => Err(e),
        Ok(entry) => Ok(cache_passwd_to_nss_passwd(entry)),
    }
}

// result_vec_cache_passwd_to_result_vec_nss_passwd converts a list of cache password entries to a list of NSS password entries.
fn result_vec_cache_passwd_to_result_vec_nss_passwd(
    entry: Result<Vec<CachePasswd>, CacheError>,
) -> Result<Vec<Passwd>, CacheError> {
    let mut r = vec![];
    match entry {
        Err(e) => Err(e),
        Ok(entries) => {
            for e in entries {
                r.push(cache_passwd_to_nss_passwd(e));
            }
            Ok(r)
        }
    }
}
