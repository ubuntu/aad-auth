#[macro_use]
extern crate lazy_static; // used by libnss_*_hooks macros
#[macro_use]
extern crate libnss;
use libnss::interop::Response;

mod passwd;
use passwd::AADPasswd;
libnss_passwd_hooks!(aad, AADPasswd);

mod cache;
use crate::cache::{CacheDB, CacheError};

use crate::logs::LOGPREFIX;
mod logs;

use std::env;

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

fn new_cache() -> Result<CacheDB, CacheError> {
    let mut c = CacheDB::new();

    if cfg!(feature = "integration_tests") {
        let cache_dir = env::var("NSS_AAD_CACHEDIR").unwrap_or("".to_string());
        if cache_dir != "" {
            c.with_db_path(&cache_dir);
        }
    }

    c.build()
}
