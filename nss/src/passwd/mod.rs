use libc::uid_t;

use libnss::interop::Response;
use libnss::passwd::{Passwd, PasswdHooks};

use crate::cache::{CacheError, Passwd as CachePasswd};
use crate::{cache_result_to_nss_status, new_cache};

use crate::{debug, LOGPREFIX};

pub struct AADPasswd;

impl PasswdHooks for AADPasswd {
    fn get_all_entries() -> Response<Vec<Passwd>> {
        debug!("get_all_entries for passwd");

        let c = match new_cache() {
            Ok(c) => c,
            Err(e) => return cache_result_to_nss_status(Err(e)),
        };

        let r = result_vec_cache_passwd_to_result_vec_nss_passwd(c.get_all_passwd());
        return cache_result_to_nss_status(r);
    }

    fn get_entry_by_uid(uid: uid_t) -> Response<Passwd> {
        debug!("get_entry_by_uid for passwd for uid: {}", uid);

        let c = match new_cache() {
            Ok(c) => c,
            Err(e) => return cache_result_to_nss_status(Err(e)),
        };

        let r = result_cache_passwd_to_result_nss_passwd(c.get_passwd_from_uid(uid));
        return cache_result_to_nss_status(r);
    }

    fn get_entry_by_name(name: String) -> Response<Passwd> {
        debug!("get_entry_by_name for passwd for name: {}", name);

        let c = match new_cache() {
            Ok(c) => c,
            Err(e) => return cache_result_to_nss_status(Err(e)),
        };

        let r = result_cache_passwd_to_result_nss_passwd(c.get_passwd_from_name(&name));
        return cache_result_to_nss_status(r);
    }
}

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

fn result_cache_passwd_to_result_nss_passwd(
    entry: Result<CachePasswd, CacheError>,
) -> Result<Passwd, CacheError> {
    match entry {
        Err(e) => Err(e),
        Ok(entry) => Ok(cache_passwd_to_nss_passwd(entry)),
    }
}

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
