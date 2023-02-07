#[cfg(feature = "integration-tests")]
use cache::CacheDBBuilder;
#[cfg(feature = "integration-tests")]
use ctor::ctor;
#[cfg(feature = "integration-tests")]
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
use logs::init_logger;

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

    #[cfg(feature = "integration-tests")]
    override_cache_options(&mut c);

    c.build()
}

/// override_cache_options parses the NSS_AAD env variables and overrides the cache default options
/// according to the specified values.
#[cfg(feature = "integration-tests")]
fn override_cache_options(c: &mut CacheDBBuilder) {
    debug!("Overriding cache options for tests");

    if let Ok(cache_dir) = env::var("NSS_AAD_CACHEDIR") {
        c.with_db_path(&cache_dir);
    }

    if let Ok(root_uid) = env::var("NSS_AAD_ROOT_UID") {
        c.with_root_uid(root_uid.parse().unwrap());
    }

    if let Ok(root_gid) = env::var("NSS_AAD_ROOT_GID") {
        c.with_root_gid(root_gid.parse().unwrap());
    }

    if let Ok(shadow_gid) = env::var("NSS_AAD_SHADOW_GID") {
        c.with_shadow_gid(shadow_gid.parse().unwrap());
    }

    if let Ok(shadow_mode) = env::var("NSS_AAD_SHADOWMODE") {
        c.with_shadow_mode(shadow_mode.parse().unwrap());
    }
}

#[cfg(feature = "integration-tests")]
#[ctor]
/// register_local_aad_nss_service_for_tests executes the C API to override the NSS lookup.
fn register_local_aad_nss_service_for_tests() {
    #[link(name = "db_override")]
    extern "C" {
        fn db_override();
    }

    unsafe {
        db_override();
    }
}

#[cfg(test)]
mod testutils;