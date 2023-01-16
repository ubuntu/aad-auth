use std::env;

#[macro_use]
extern crate lazy_static; // used by libnss_*_hooks macros
use libnss::{interop::Response, libnss_passwd_hooks};

mod passwd;
use passwd::AADPasswd;
libnss_passwd_hooks!(aad, AADPasswd);

mod cache;
use crate::cache::{CacheDB, CacheError};

mod logs;
use crate::logs::LOGPREFIX;

// cache_result_to_nss_status converts our internal CacheError to a nss-compatible Response.
fn cache_result_to_nss_status<T>(r: Result<T, CacheError>) -> Response<T> {
    match r {
        Ok(t) => Response::Success(t),
        Err(e) => match e {
            CacheError::DatabaseError(e) => {
                error!("database error: {}", e);
                Response::Unavail
            }
            CacheError::QueryError(e) => {
                error!("query error: {}", e);
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
        #[allow(clippy::or_fun_call)]
        let cache_dir = env::var("NSS_AAD_CACHEDIR").unwrap_or("".to_string());
        if !cache_dir.is_empty() {
            c.with_db_path(&cache_dir);
        }
    }

    c.build()
}

#[cfg(test)]
mod testutils;
