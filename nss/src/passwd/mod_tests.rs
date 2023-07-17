use std::collections::BTreeMap;

use super::AADPasswd;
use crate::testutils;
use libnss::{
    interop::NssStatus,
    interop::Response,
    passwd::{Passwd, PasswdHooks},
};
use test_case::test_case;

#[test_case(Some("users_in_db".to_string()), -1, NssStatus::Success; "Successfully retrieves all entries")]
#[test_case(Some("users_in_db".to_string()), -1, NssStatus::Success; "Successfully retrieves all entries without access to shadow")]
#[test_case(Some("empty".to_string()), -1, NssStatus::Success; "Does not error out when the cache is empty")]
#[test_case(Some("no_cache".to_string()), -1, NssStatus::Unavail; "Error when cache is not available")]
fn test_get_all_entries(
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_status: NssStatus,
) {
    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

    std::env::set_var("NSS_AAD_CACHEDIR", cache_dir.path().to_str().unwrap());
    if force_shadow_mode > -1 {
        std::env::set_var("NSS_AAD_SHADOW_MODE", force_shadow_mode.to_string());
    }

    let got = AADPasswd::get_all_entries();
    assert!(
        got.to_status() == want_status,
        "Expected {:?}, but got {:?}",
        want_status,
        got.to_status()
    );

    let value = match got {
        Response::Success(v) => v,
        _ => return,
    };

    let mut parsed_values: Vec<BTreeMap<&str, String>> = Vec::new();
    for entry in value {
        parsed_values.push(nss_passwd_to_bmap(entry));
    }

    let module_path = testutils::get_module_path(file!());
    testutils::load_and_update_golden(&module_path, parsed_values);
}

#[test_case(1929326240, Some("users_in_db".to_string()), -1, NssStatus::Success; "Successfully retrieves existing entry")]
#[test_case(1929326240, Some("users_in_db".to_string()), 0, NssStatus::Success; "Successfully retrieves existing entry without access to shadow")]
#[test_case(4242, Some("users_in_db".to_string()), -1, NssStatus::NotFound; "Error when entry does not exist")]
#[test_case(1929326240, Some("no_cache".to_string()), -1, NssStatus::Unavail; "Error when cache is not available")]
fn test_get_entry_by_uid(
    uid: u32,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_status: NssStatus,
) {
    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

    std::env::set_var("NSS_AAD_CACHEDIR", cache_dir.path().to_str().unwrap());
    if force_shadow_mode > -1 {
        std::env::set_var("NSS_AAD_SHADOW_MODE", force_shadow_mode.to_string());
    }

    let got = AADPasswd::get_entry_by_uid(uid);
    assert!(
        got.to_status() == want_status,
        "Expected {:?}, but got {:?}",
        want_status,
        got.to_status()
    );

    let entry = match got {
        Response::Success(v) => v,
        _ => return,
    };

    let module_path = testutils::get_module_path(file!());
    testutils::load_and_update_golden(&module_path, nss_passwd_to_bmap(entry));
}

#[test_case("myuser@domain.com", Some("users_in_db".to_string()), -1, NssStatus::Success; "Successfully retrieves existing entry")]
#[test_case("MyUser@Domain.Com", Some("users_in_db".to_string()), -1, NssStatus::Success; "Successfully retrieves existing entry with capitalized letters")]
#[test_case("myuser@domain.com", Some("users_in_db".to_string()), -1, NssStatus::Success; "Successfully retrieves existing entry without access to shadow")]
#[test_case("does not exist", Some("users_in_db".to_string()), -1, NssStatus::NotFound; "Error when entry does not exist")]
#[test_case("myuser@domain.com", Some("no_cache".to_string()), -1, NssStatus::Unavail; "Error when cache is not available")]
fn test_get_entry_by_name(
    name: &str,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_status: NssStatus,
) {
    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

    std::env::set_var("NSS_AAD_CACHEDIR", cache_dir.path().to_str().unwrap());
    if force_shadow_mode > -1 {
        std::env::set_var("NSS_AAD_SHADOW_MODE", force_shadow_mode.to_string());
    }

    let got = AADPasswd::get_entry_by_name(name.to_string());
    assert!(
        got.to_status() == want_status,
        "Expected {:?}, but got {:?}",
        want_status,
        got.to_status()
    );

    let entry = match got {
        Response::Success(v) => v,
        _ => return,
    };

    let module_path = testutils::get_module_path(file!());
    testutils::load_and_update_golden(&module_path, nss_passwd_to_bmap(entry));
}

/// nss_passwd_to_bmap transforms a libnss::Passwd struct into a BTreeMap
fn nss_passwd_to_bmap(entry: Passwd) -> BTreeMap<&'static str, String> {
    let mut parsed_entry = BTreeMap::new();
    parsed_entry.insert("name", entry.name);
    parsed_entry.insert("passwd", entry.passwd);
    parsed_entry.insert("uid", entry.uid.to_string());
    parsed_entry.insert("gid", entry.gid.to_string());
    parsed_entry.insert("gecos", entry.gecos);
    parsed_entry.insert("dir", entry.dir);
    parsed_entry.insert("shell", entry.shell);

    parsed_entry
}
