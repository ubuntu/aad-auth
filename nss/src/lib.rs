use std::env;

#[macro_use]
extern crate lazy_static; // used by libnss_*_hooks macros
use libnss::{interop::Response, libnss_group_hooks, libnss_passwd_hooks, libnss_shadow_hooks};

mod passwd;
use passwd::AADPasswd;
libnss_passwd_hooks!(aad, AADPasswd);

mod group;
use group::AADGroup;
libnss_group_hooks!(aad, AADGroup);

mod shadow;
use shadow::AADShadow;
libnss_shadow_hooks!(aad, AADShadow);

mod cache;
use crate::cache::{CacheDB, CacheError};

mod logs;
use crate::logs::LOGPREFIX;

// cache_result_to_nss_status converts our internal CacheError to a nss-compatible Response.
fn cache_result_to_nss_status<T>(r: Result<T, CacheError>) -> Response<T> {
    match r {
        Ok(t) => Response::Success(t),
        Err(err) => match err {
            CacheError::DatabaseError(err) => {
                error!("database error: {}", err);
                Response::Unavail
            }
            CacheError::QueryError(err) => {
                error!("query error: {}", err);
                Response::Unavail
            }
            CacheError::NoRecord => {
                debug!("no record found");
                Response::NotFound
            }
        },
    }
}

// new_cache initializes the cache with an optional cache directory for integration testing.
fn new_cache() -> Result<CacheDB, CacheError> {
    let mut c = CacheDB::new();

    if cfg!(feature = "integration_tests") {
        let cache_dir = env::var("NSS_AAD_CACHEDIR").unwrap_or_default();
        if !cache_dir.is_empty() {
            c.with_db_path(&cache_dir);
        }
    }

    c.build()
}

#[cfg(test)]
mod testutils;
