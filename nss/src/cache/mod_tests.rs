use std::fs::{self, Permissions};
use std::os::unix::prelude::PermissionsExt;

use test_case::test_case;

use crate::testutils;
use crate::CacheDB;

#[test_case(None, None, None, None, None, None, None, None, -1, Some("users_in_db".to_string()), false; "Successfully opens cache with default values")]
#[test_case(None, None, None, None, None, None, None, Some(0o550), 1, Some("users_in_db".to_string()), false; "Successfully opens cache when dir has no write perms")]
#[test_case(None, None, None, None, Some(0o000), None, Some(0o000), None, -1, Some("users_in_db".to_string()), false; "Successfully opens cache without shadow if current user has no shadow access")]
#[test_case(Some(1234), None, None, None, None, None, None, None, -1, Some("users_in_db".to_string()), true; "Error when cache has invalid owner uid")]
#[test_case(None, Some(1234), None, None, None, None,None, None, -1, Some("users_in_db".to_string()), true; "Error when cache has invalid owner gid")]
#[test_case(None, None, Some(1234), None, None, None, None, None, -1, Some("users_in_db".to_string()), true; "Error when cache has invalid shadow gid")]
#[test_case(None, None, None, Some(0o444), None, None, None, None, -1, Some("users_in_db".to_string()), true; "Error when passwd.db has invalid permissions")]
#[test_case(None, None, None, Some(0o000), None, Some(0o000), None, None, -1, Some("users_in_db".to_string()), true; "Error when current user has no access to passwd")]
#[test_case(None, None, None, None, Some(0o444), None, None, None, -1, Some("users_in_db".to_string()), true; "Error when shadow.db has invalid permissions")]
#[test_case(None, None, None, None, None, None, None, Some(0o444), 2, Some("users_in_db".to_string()), true; "Error when cache dir has RO perms and shadow mode is RW")]
#[test_case(None, None, None, None, None, None, None, None, -1, Some("no_cache".to_string()), true; "Error when there is no cache")]
#[test_case(None, None, None, None, None, None, None, None, -1, Some("passwd_only".to_string()), true; "Error when there is only passwd")]
#[test_case(None, None, None, None, None, None, None, None, -1, Some("shadow_only".to_string()), true; "Error when there is only shadow")]
fn test_build(
    root_uid: Option<u32>,
    root_gid: Option<u32>,
    shadow_gid: Option<u32>,
    passwd_creation_perms: Option<u32>,
    shadow_creation_perms: Option<u32>,
    passwd_expected_perms: Option<u32>,
    shadow_expected_perms: Option<u32>,
    cache_dir_perms: Option<u32>,
    force_shadow_mode: i32,
    initial_state: Option<String>,
    want_err: bool,
) {
    let mut opts = vec![testutils::with_initial_state(initial_state)];
    if let Some(mode) = passwd_creation_perms {
        opts.push(testutils::with_passwd_perms(mode));
    }
    if let Some(mode) = shadow_creation_perms {
        opts.push(testutils::with_shadow_perms(mode));
    }

    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

    if let Some(mode) = cache_dir_perms {
        let r = fs::set_permissions(&cache_dir, Permissions::from_mode(mode));
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
    if let Some(mode) = passwd_expected_perms {
        builder = builder.with_passwd_perms(mode);
    }
    if let Some(mode) = shadow_expected_perms {
        builder = builder.with_shadow_perms(mode);
    }

    let got = builder.build();
    if want_err {
        testutils::require_error(got.as_ref(), "build");
        return;
    }
    testutils::require_no_error(got.as_ref(), "build");
}

/* PASSWD TESTS */
#[test_case(165119649, Some("users_in_db".to_string()), -1, false  ; "Get existing user by uid")]
#[test_case(165119649, Some("users_in_db".to_string()), 0, false  ; "Get existing user by uid without access to shadow")]
#[test_case(4242, Some("users_in_db".to_string()), -1, true  ; "Error when user does not exist")]
fn test_get_passwd_by_uid(
    uid: u32,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());

    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

    let (current_uid, current_gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(current_uid)
        .with_root_gid(current_gid)
        .with_shadow_gid(current_gid)
        .with_shadow_mode(force_shadow_mode)
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

#[test_case("myuser@domain.com", Some("users_in_db".to_string()), -1, false; "Get existing user by name")]
#[test_case("myuser@domain.com", Some("users_in_db".to_string()), 0, false; "Get existing user by name without access to shadow")]
#[test_case("does not exist", Some("users_in_db".to_string()), -1, true; "Error when user does not exist")]
fn test_get_passwd_by_name(
    name: &str,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());

    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

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
        testutils::require_error(got.as_ref(), "get_passwd_by_name");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_passwd_by_name");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case(90, Some("db_with_expired_users".to_string()), -1, false ; "Get all entries cleaning up entries to purge")]
#[test_case(0, Some("db_with_expired_users".to_string()), -1, false ; "Get all entries without cleaning up when offline expiration is disabled")]
#[test_case(90, Some("db_with_expired_users".to_string()), 1, false ; "Get all entries without cleaning when ShadowMode is less than RW")]
fn test_get_all_passwd(
    credentials_expiration: i32,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());
    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

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
        testutils::require_error(got.as_ref(), "get_all_passwds");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_all_passwds");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

/* GROUP TESTS */
#[test_case(1929326240, Some("users_in_db".to_string()), -1, false; "Get existing group by gid")]
#[test_case(1929326240, Some("users_in_db".to_string()), 0, false; "Get existing group by gid without access to shadow")]
#[test_case(4242, Some("users_in_db".to_string()), -1, true; "Error when group does not exist")]
fn test_get_group_by_gid(
    gid: u32,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());

    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

    let (current_uid, current_gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(current_uid)
        .with_root_gid(current_gid)
        .with_shadow_gid(current_gid)
        .with_shadow_mode(force_shadow_mode)
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

#[test_case("myuser@domain.com", Some("users_in_db".to_string()), -1, false; "Get existing group by name")]
#[test_case("myuser@domain.com", Some("users_in_db".to_string()), 0, false; "Get existing group by name without access to shadow")]
#[test_case("does not exist", Some("users_in_db".to_string()), -1, true; "Error when group does not exist")]
fn test_get_group_by_name(
    name: &str,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());

    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

    let (uid, gid) = (users::get_current_uid(), users::get_current_gid());
    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .with_root_uid(uid)
        .with_root_gid(gid)
        .with_shadow_gid(gid)
        .with_shadow_mode(force_shadow_mode)
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_group_by_name(name);
    if want_err {
        testutils::require_error(got.as_ref(), "get_group_by_name");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_group_by_name");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case(90, Some("db_with_expired_users".to_string()), -1, false ; "Get all entries cleaning up entries to purge")]
#[test_case(0, Some("db_with_expired_users".to_string()), -1, false ; "Get all entries without cleaning up when offline expiration is disabled")]
#[test_case(90, Some("db_with_expired_users".to_string()), 1, false ; "Get all entries without cleaning when ShadowMode is less than RW")]
fn test_get_all_groups(
    credentials_expiration: i32,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());

    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

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
        testutils::require_error(got.as_ref(), "get_all_groups");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_all_groups");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

/* SHADOW TESTS */
#[test_case("myuser@domain.com", Some("users_in_db".to_string()), -1, false ; "Get existing shadow by name")]
#[test_case("does not exist", Some("users_in_db".to_string()), -1, true ; "Error when user does not exist")]
#[test_case("myuser@domain.com", Some("users_in_db".to_string()), 0, true ; "Error when shadow is unavailable")]
fn test_get_shadow_by_name(
    name: &str,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());

    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

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
        testutils::require_error(got.as_ref(), "get_shadow_by_name");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_shadow_by_name");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}

#[test_case(90, Some("db_with_expired_users".to_string()), -1, false ; "Get all entries cleaning up entries to purge")]
#[test_case(0, Some("db_with_expired_users".to_string()), -1, false ; "Get all entries without cleaning up when offline expiration is disabled")]
#[test_case(90, Some("db_with_expired_users".to_string()), 1, false ; "Get all entries without cleaning when ShadowMode is less than RW")]
#[test_case(90, Some("db_with_expired_users".to_string()), 0, true ; "Error when shadow is unavailable")]
fn test_get_all_shadows(
    credentials_expiration: i32,
    initial_state: Option<String>,
    force_shadow_mode: i32,
    want_err: bool,
) {
    let module_path = testutils::get_module_path(file!());

    let opts = vec![testutils::with_initial_state(initial_state)];
    let cache_dir = testutils::prepare_db_for_tests(opts)
        .expect("Setup: failed to prepare db for tests")
        .unwrap();

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
        testutils::require_error(got.as_ref(), "get_all_shadows");
        return;
    }
    testutils::require_no_error(got.as_ref(), "get_all_shadows");
    testutils::load_and_update_golden(&module_path, got.unwrap());
}
