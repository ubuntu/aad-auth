use std::fs::{self, Permissions};
use std::os::unix::prelude::PermissionsExt;

use tempfile::TempDir;
use test_case::test_case;

use crate::testutils;
use crate::CacheDB;

#[test_case(None, None, None, None, None, None, -1, Some("users_in_db".to_string()), false; "Successfully opens cache with default values")]
#[test_case(None, None, None, None, None, Some(0o550), 1, Some("users_in_db".to_string()), false; "Successfully opens cache when dir has no write perms")]
#[test_case(Some(1234), None, None, None, None, None, -1, Some("users_in_db".to_string()), true; "Error when cache has invalid owner uid")]
#[test_case(None, Some(1234), None, None, None, None, -1, Some("users_in_db".to_string()), true; "Error when cache has invalid owner gid")]
#[test_case(None, None, Some(1234), None, None, None, -1, Some("users_in_db".to_string()), true; "Error when cache has invalid shadow gid")]
#[test_case(None, None, None, Some(0o444), None, None, -1, Some("users_in_db".to_string()), true; "Error when passwd.db has invalid permissions")]
#[test_case(None, None, None, None, Some(0o444), None, -1, Some("users_in_db".to_string()), true; "Error when shadow.db has invalid permissions")]
#[test_case(None, None, None, None, None, Some(0o444), 2, Some("users_in_db".to_string()), true; "Error when cache dir has RO perms and shadow mode is RW")]
#[test_case(None, None, None, None, None, Some(0o444), 2, None, true; "Error when cache dir has RO perms, shadow mode is RW and there is no cache")]
fn test_build(
    root_uid: Option<u32>,
    root_gid: Option<u32>,
    shadow_gid: Option<u32>,
    passwd_perms: Option<u32>,
    shadow_perms: Option<u32>,
    cache_dir_perms: Option<u32>,
    force_shadow_mode: i32,
    initial_state: Option<String>,
    want_err: bool,
) {
    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    let mut opts = vec![testutils::with_initial_state(initial_state)];
    if let Some(mode) = passwd_perms {
        opts.push(testutils::with_passwd_perms(mode));
    }
    if let Some(mode) = shadow_perms {
        opts.push(testutils::with_shadow_perms(mode));
    }

    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {err:?}");
    }

    if let Some(mode) = cache_dir_perms {
        let r = fs::set_permissions(cache_dir.path(), Permissions::from_mode(mode));
        println!("permissions set to {mode:o}");
        r.unwrap_or_else(|_| {
            panic!("Setup: Failed to set requested permissions {mode} for cache_dir")
        });
    }

    let mut builder = &mut CacheDB::new();
    builder
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_shadow_mode(force_shadow_mode)
        .with_root_uid(users::get_current_uid())
        .with_root_gid(users::get_current_gid())
        .with_shadow_gid(users::get_current_gid());

    if let Some(uid) = root_uid {
        builder = builder.with_root_uid(uid);
    }
    if let Some(gid) = root_gid {
        builder = builder.with_root_gid(gid);
    }
    if let Some(gid) = shadow_gid {
        builder = builder.with_shadow_gid(gid);
    }

    let got = builder.build();
    if want_err {
        testutils::require_error(got.as_ref());
        return;
    }
    testutils::require_no_error(got.as_ref());
}

/* PASSWD TESTS */
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
        testutils::require_error(got.as_ref());
        return;
    }
    testutils::require_no_error(got.as_ref());
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case("myuser@domain.com", 1, Some("users_in_db".to_string()), false; "Get existing user by name")]
#[test_case("does not exist", -1, None, true; "Error on non existing user")]
fn test_get_passwd_by_name(
    name: &str,
    force_shadow_mode: i32,
    initial_state: Option<String>,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());
    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    let opts = vec![testutils::with_initial_state(initial_state)];
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {err:?}");
    }

    let (uid, gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(uid)
        .with_root_gid(gid)
        .with_shadow_gid(gid)
        .with_shadow_mode(force_shadow_mode)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_passwd_by_name(name);
    if want_err {
        testutils::require_error(got.as_ref());
        return;
    }
    testutils::require_no_error(got.as_ref());
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case(90, 2, Some("db_with_expired_users".to_string()), false ; "Get all entries cleaning up entries to purge")]
#[test_case(0, 2, Some("db_with_expired_users".to_string()), false ; "Get all entries without cleaning up when offline expiration is disabled")]
#[test_case(90, 1, Some("db_with_expired_users".to_string()), false ; "Get all entries without cleaning when ShadowMode is less than 2")]
fn test_get_all_passwd(
    credentials_expiration: i32,
    force_shadow_mode: i32,
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
        .with_shadow_mode(force_shadow_mode)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_all_passwds();
    if want_err {
        testutils::require_error(got.as_ref());
        return;
    }
    testutils::require_no_error(got.as_ref());
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

/* GROUP TESTS */
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
        testutils::require_error(got.as_ref());
        return;
    }
    testutils::require_no_error(got.as_ref());
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case("myuser@domain.com", Some("users_in_db".to_string()), false; "Get existing group by name")]
#[test_case("non existent", None, true; "Error on non existing group")]
fn test_get_group_by_name(name: &str, initial_state: Option<String>, want_err: bool) {
    let module_path = testutils::get_module_path(file!());
    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    let opts = vec![testutils::with_initial_state(initial_state)];
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {err:?}")
    }

    let (uid, gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(uid)
        .with_root_gid(gid)
        .with_shadow_gid(gid)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_group_by_name(name);
    if want_err {
        testutils::require_error(got.as_ref());
        return;
    }
    testutils::require_no_error(got.as_ref());
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case(90, 2, Some("db_with_expired_users".to_string()), false ; "Get all entries cleaning up entries to purge")]
#[test_case(0, 2, Some("db_with_expired_users".to_string()), false ; "Get all entries without cleaning up when offline expiration is disabled")]
#[test_case(90, 1, Some("db_with_expired_users".to_string()), false ; "Get all entries without cleaning when ShadowMode is less than 2")]
fn test_get_all_groups(
    credentials_expiration: i32,
    force_shadow_mode: i32,
    initial_state: Option<String>,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());
    let cache_dir = TempDir::new().expect("Could not create temporary directory");

    let opts = vec![testutils::with_initial_state(initial_state)];
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {err:?}")
    }

    let (uid, gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(uid)
        .with_root_gid(gid)
        .with_shadow_gid(gid)
        .with_offline_credentials_expiration(credentials_expiration)
        .with_shadow_mode(force_shadow_mode)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_all_groups();
    if want_err {
        testutils::require_error(got.as_ref());
        return;
    }
    testutils::require_no_error(got.as_ref());
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

/* SHADOW TESTS */
#[test_case("myuser@domain.com", -1, Some("users_in_db".to_string()), false ; "Get existing shadow entry")]
#[test_case("unexistent", -1, Some("users_in_db".to_string()), true ; "Error on non existing user")]
#[test_case("myuser@domain.com", 0, Some("users_in_db".to_string()), true ; "Error when shadow is unavailable")]
fn test_get_shadow_by_name(
    name: &str,
    force_shadow_mode: i32,
    initial_state: Option<String>,
    want_err: bool,
) {
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
        .with_shadow_mode(force_shadow_mode)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_shadow_by_name(name);
    if want_err {
        testutils::require_error(got.as_ref());
        return;
    }
    testutils::require_no_error(got.as_ref());
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case(90, 2, Some("db_with_expired_users".to_string()), false ; "Get all entries cleaning up entries to purge")]
#[test_case(0, 2, Some("db_with_expired_users".to_string()), false ; "Get all entries without cleaning up when offline expiration is disabled")]
#[test_case(90, 1, Some("db_with_expired_users".to_string()), false ; "Get all entries without cleaning when ShadowMode is less than 2")]
#[test_case(90, 0, Some("db_with_expired_users".to_string()), true ; "Error when shadow is unavailable")]
fn test_get_all_shadows(
    credentials_expiration: i32,
    force_shadow_mode: i32,
    initial_state: Option<String>,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());
    let cache_dir = TempDir::new().expect("Could not create temporary directory");

    let opts = vec![testutils::with_initial_state(initial_state)];
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {err:?}")
    }

    let (uid, gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(uid)
        .with_root_gid(gid)
        .with_shadow_gid(gid)
        .with_offline_credentials_expiration(credentials_expiration)
        .with_shadow_mode(force_shadow_mode)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_all_shadows();
    if want_err {
        testutils::require_error(got.as_ref());
        return;
    }
    testutils::require_no_error(got.as_ref());
    testutils::load_and_update_golden(&module_path, got.unwrap());
}
