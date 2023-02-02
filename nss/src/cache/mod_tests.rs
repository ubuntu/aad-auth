use tempfile::TempDir;
use test_case::test_case;

use crate::testutils;
use crate::CacheDB;

#[test_case(165119649, Some("users_in_db".to_string()), false  ; "Get existing user")]
#[test_case(4242, Some("users_in_db".to_string()), true  ; "Error on non existing user")]
fn test_get_passwd_by_uid(uid: u32, initial_state: Option<String>, want_err: bool) {
    let module_path = testutils::get_module_path(file!());

    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    let opts = vec![testutils::with_initial_state(initial_state)];
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {err:?}")
    }

    let (current_uid, current_gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(current_uid)
        .with_root_gid(current_gid)
        .with_shadow_gid(current_gid)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_passwd_by_uid(uid);
    if want_err {
        testutils::require_error(got.as_ref(), "get_passwd_by_uid");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_passwd_by_uid");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case(90, 2, Some("db_with_expired_users".to_string()), false ; "Get all entries cleaning up entries to purge")]
#[test_case(0, 2, Some("db_with_expired_users".to_string()), false ; "Get all entries without cleaning up when offline expiration is disabled")]
#[test_case(90, 1, Some("db_with_expired_users".to_string()), false ; "Get all entries without cleaning when ShadowMode is less than 2")]
fn test_get_all_passwd(
    credentials_expiration: i32,
    shadow_mode: i32,
    initial_state: Option<String>,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());
    let cache_dir = TempDir::new().expect("Could not create temporary directory");

    let opts = vec![testutils::with_initial_state(initial_state)];
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {err:?}")
    }

    let (current_uid, current_gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(current_uid)
        .with_root_gid(current_gid)
        .with_shadow_gid(current_gid)
        .with_offline_credentials_expiration(credentials_expiration)
        .with_shadow_mode(shadow_mode)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_all_passwds();
    if want_err {
        testutils::require_error(got.as_ref(), "get_all_passwds");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_all_passwds");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case(1929326240, Some("users_in_db".to_string()), false; "Get existing group")]
#[test_case(4242, Some("users_in_db".to_string()), true; "Error on non existing group")]
fn test_get_group_by_gid(gid: u32, initial_state: Option<String>, want_err: bool) {
    let module_path = testutils::get_module_path(file!());

    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    let opts = vec![testutils::with_initial_state(initial_state)];
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {err:?}")
    }

    let (current_uid, current_gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(current_uid)
        .with_root_gid(current_gid)
        .with_shadow_gid(current_gid)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_group_by_gid(gid);
    if want_err {
        testutils::require_error(got.as_ref(), "get_group_by_gid");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_group_by_gid");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case("myuser@domain.com", Some("users_in_db".to_string()), false ; "Get existing shadow entry")]
#[test_case("unexistent", Some("users_in_db".to_string()), true ; "Error on non existing user")]
fn test_get_shadow_by_name(name: &str, initial_state: Option<String>, want_err: bool) {
    let module_path = testutils::get_module_path(file!());

    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    let opts = vec![testutils::with_initial_state(initial_state)];
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {err:?}")
    }

    let (current_uid, current_gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(current_uid)
        .with_root_gid(current_gid)
        .with_shadow_gid(current_gid)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_shadow_by_name(name);
    if want_err {
        testutils::require_error(got.as_ref(), "get_shadow_by_name");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_shadow_by_name");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}
