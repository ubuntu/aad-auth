use std::collections::BTreeMap;

use super::AADShadow;
use crate::testutils;
use libnss::{
    interop::NssStatus,
    interop::Response,
    shadow::{Shadow, ShadowHooks},
};
use test_case::test_case;

#[test_case(Some("users_in_db".to_string()), 1, NssStatus::Success; "Successfully retrieves all entries in RO mode")]
#[test_case(Some("users_in_db".to_string()), 2, NssStatus::Success; "Successfully retrieves all entries in RW mode")]
#[test_case(Some("empty".to_string()), 1, NssStatus::Success; "Does not error out when cache is empty")]
#[test_case(Some("no_cache".to_string()), -1, NssStatus::Unavail; "Error when cache is not available")]
#[test_case(Some("users_in_db".to_string()), 0, NssStatus::Unavail; "Error when shadow is not available")]
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

    let got = AADShadow::get_all_entries();
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
        parsed_values.push(nss_shadow_to_bmap(entry));
    }

    let module_path = testutils::get_module_path(file!());
    testutils::load_and_update_golden(&module_path, parsed_values);
}

#[test_case("myuser@domain.com", Some("users_in_db".to_string()), 1, NssStatus::Success; "Successfully retrieves existing entry in RO mode")]
#[test_case("MyUser@Domain.Com", Some("users_in_db".to_string()), -1, NssStatus::Success; "Successfully retrieves existing entry with capitalized letters in RO mode")]
#[test_case("myuser@domain.com", Some("users_in_db".to_string()), 2, NssStatus::Success; "Successfully retrieves existing entry in RW mode")]
#[test_case("MyUser@Domain.Com", Some("users_in_db".to_string()), -1, NssStatus::Success; "Successfully retrieves existing entry with capitalized letters in RW mode")]
#[test_case("does not exist", Some("users_in_db".to_string()), 1, NssStatus::NotFound; "Error when entry does not exist")]
#[test_case("myuser@domain.com", Some("no_cache".to_string()), 1, NssStatus::Unavail; "Error when cache is not available")]
#[test_case("myuser@domain.com", Some("users_in_db".to_string()), 0, NssStatus::Unavail; "Error when shadow is not available")]
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

    let got = AADShadow::get_entry_by_name(name.to_string());
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
    testutils::load_and_update_golden(&module_path, nss_shadow_to_bmap(entry));
}

/// nss_shadow_to_bmap transforms a libnss::Shadow struct into a BTreeMap
fn nss_shadow_to_bmap(entry: Shadow) -> BTreeMap<&'static str, String> {
    let mut parsed_entry = BTreeMap::new();
    parsed_entry.insert("name", entry.name);
    parsed_entry.insert("passwd", entry.passwd);
    parsed_entry.insert("last_change", entry.last_change.to_string());
    parsed_entry.insert("change_min_days", entry.change_min_days.to_string());
    parsed_entry.insert("change_max_days", entry.change_max_days.to_string());
    parsed_entry.insert("change_warn_days", entry.change_warn_days.to_string());
    parsed_entry.insert(
        "change_inactive_days",
        entry.change_inactive_days.to_string(),
    );
    parsed_entry.insert("expire_date", entry.expire_date.to_string());
    // The reserved field value is arch dependent, which means its value will change depending on the
    // build, so we should use a symbolic value to avoid diff when building on 32 and 64 archs.
    parsed_entry.insert("reserved", "RESERVED".to_string());

    parsed_entry
}
