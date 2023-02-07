use std::collections::BTreeMap;

use super::AADGroup;
use crate::testutils;
use libnss::{
    group::{Group, GroupHooks},
    interop::NssStatus,
    interop::Response,
};
use tempfile::TempDir;
use test_case::test_case;

#[test_case(Some("users_in_db".to_string()), false, -1, NssStatus::Success; "Successfully retrieves all entries")]
#[test_case(Some("users_in_db".to_string()), false, 0, NssStatus::Success; "Successfully retrieves all entries without access to shadow")]
#[test_case(None, false, -1, NssStatus::Success; "Does not error out when the cache is empty")]
#[test_case(None, true, -1, NssStatus::Unavail; "Error when cache is not available")]
fn test_get_all_entries(
    initial_state: Option<String>,
    no_cache: bool,
    force_shadow_mode: i32,
    want_status: NssStatus,
) {
    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    if !no_cache {
        let opts = vec![testutils::with_initial_state(initial_state)];
        if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
            panic!("Setup: Failed to prepare db for tests: {err:?}");
        }
    }

    std::env::set_var("NSS_AAD_CACHEDIR", cache_dir.path().to_str().unwrap());
    if force_shadow_mode > -1 {
        std::env::set_var("NSS_AAD_SHADOW_MODE", force_shadow_mode.to_string());
    }

    let got = AADGroup::get_all_entries();
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
        parsed_values.push(nss_group_to_bmap(entry));
    }

    let module_path = testutils::get_module_path(file!());
    testutils::load_and_update_golden(&module_path, parsed_values);
}

#[test_case(1929326240, Some("users_in_db".to_string()), false, -1, NssStatus::Success; "Successfully retrieves existing entry")]
#[test_case(1929326240, Some("users_in_db".to_string()), false, 0, NssStatus::Success; "Successfully retrieves existing entry without access to shadow")]
#[test_case(4242, Some("users_in_db".to_string()), false, -1, NssStatus::NotFound; "Error when entry does not exist")]
#[test_case(1929326240, None, true, -1, NssStatus::Unavail; "Error when cache is not available")]
fn test_get_entry_by_gid(
    gid: u32,
    initial_state: Option<String>,
    no_cache: bool,
    force_shadow_mode: i32,
    want_status: NssStatus,
) {
    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    if !no_cache {
        let opts = vec![testutils::with_initial_state(initial_state)];
        if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
            panic!("Setup: Failed to prepare db for tests: {err:?}");
        }
    }

    std::env::set_var("NSS_AAD_CACHEDIR", cache_dir.path().to_str().unwrap());
    if force_shadow_mode > -1 {
        std::env::set_var("NSS_AAD_SHADOW_MODE", force_shadow_mode.to_string());
    }

    let got = AADGroup::get_entry_by_gid(gid);
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
    testutils::load_and_update_golden(&module_path, nss_group_to_bmap(entry));
}

#[test_case("myuser@domain.com", Some("users_in_db".to_string()), false, -1, NssStatus::Success; "Successfully retrieves existing entry")]
#[test_case("myuser@domain.com", Some("users_in_db".to_string()), false, 0, NssStatus::Success; "Successfully retrieves existing entry without access to shadow")]
#[test_case("shadow", Some("users_in_db".to_string()), false, -1, NssStatus::NotFound; "Ignore non aad entry requests")]
#[test_case("does not exist", Some("users_in_db".to_string()), false, -1, NssStatus::NotFound; "Error when entry does not exist")]
#[test_case("myuser@domain.com", None, true, -1, NssStatus::Unavail; "Error when cache is not available")]
fn test_get_entry_by_name(
    name: &str,
    initial_state: Option<String>,
    no_cache: bool,
    force_shadow_mode: i32,
    want_status: NssStatus,
) {
    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    if !no_cache {
        let opts = vec![testutils::with_initial_state(initial_state)];
        if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
            panic!("Setup: Failed to prepare db for tests: {err:?}");
        }
    }

    std::env::set_var("NSS_AAD_CACHEDIR", cache_dir.path().to_str().unwrap());
    if force_shadow_mode > -1 {
        std::env::set_var("NSS_AAD_SHADOW_MODE", force_shadow_mode.to_string());
    }

    let got = AADGroup::get_entry_by_name(name.to_string());
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
    testutils::load_and_update_golden(&module_path, nss_group_to_bmap(entry));
}

/// nss_group_to_bmap transforms a libnss::Group struct into a BTreeMap
fn nss_group_to_bmap(entry: Group) -> BTreeMap<&'static str, String> {
    let mut parsed_entry = BTreeMap::new();
    parsed_entry.insert("name", entry.name);
    parsed_entry.insert("gid", entry.gid.to_string());
    parsed_entry.insert("passwd", entry.passwd);
    parsed_entry.insert("members", entry.members.join(","));

    parsed_entry
}
