use std::io::Write;

use serde_yaml::to_string;
use tempfile::TempDir;
use test_case::test_case;

use crate::testutils;
use crate::testutils::OptionalArgs;
use crate::CacheDB;

#[test_case(165119649, Some("users_in_db"), false  ; "Get existing user")]
#[test_case(4242, Some("users_in_db"), true  ; "Error on non existing user")]
fn test_get_passwd_by_uid(uid: u32, initial_state: Option<&str>, want_err: bool) {
    let module_path = testutils::get_module_path(file!());

    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    let opts = OptionalArgs {
        initial_state,
        ..Default::default()
    };
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {:?}", err);
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
    if let Err(err) = got {
        assert!(
            want_err,
            "get_passwd_from_uid should not have returned an error but did: {:?}",
            err,
        );
        return;
    }
    let got = to_string(&got.unwrap()).unwrap();

    let mut mint = testutils::golden_mint(&module_path);
    let (_, sub_test_name) = testutils::current_test_name();
    let mut golden = mint.new_goldenfile(sub_test_name.unwrap()).unwrap();
    golden
        .write_all(got.as_bytes())
        .expect("Teardown: can't write to file to compare with golden");
}

#[test_case(1929326240, Some("users_in_db"), false; "Get existing group")]
#[test_case(4242, Some("users_in_db"), true; "Error on non existing group")]
fn test_get_group_by_gid(gid: u32, initial_state: Option<&str>, want_err: bool) {
    let module_path = testutils::get_module_path(file!());

    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    let opts = OptionalArgs {
        initial_state,
        ..Default::default()
    };
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {:?}", err);
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
    if let Err(err) = got {
        assert!(
            want_err,
            "get_passwd_from_uid should not have returned an error but did: {:?}",
            err,
        );
        return;
    }
    let got = to_string(&got.unwrap()).unwrap();

    let mut mint = testutils::golden_mint(&module_path);
    let (_, sub_test_name) = testutils::current_test_name();
    let mut golden = mint.new_goldenfile(sub_test_name.unwrap()).unwrap();
    golden
        .write_all(got.as_bytes())
        .expect("Teardown: can't write to file to compare with golden");
}

#[test_case("myuser@domain.com", Some("users_in_db"), false ; "Get existing shadow entry")]
#[test_case("unexistent", Some("users_in_db"), true ; "Error on non existing user")]
fn test_get_shadow_by_name(name: &str, initial_state: Option<&str>, want_err: bool) {
    let module_path = testutils::get_module_path(file!());

    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    let opts = OptionalArgs {
        initial_state,
        ..Default::default()
    };
    if let Err(err) = testutils::prepare_db_for_tests(cache_dir.path(), opts) {
        panic!("Setup: Failed to prepare db for tests: {:?}", err)
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
    if let Err(err) = got {
        assert!(
            want_err,
            "get_shadow_by_name should not have returned an error but did: {:?}",
            err,
        );
        return;
    }
    let got = to_string(&got.unwrap()).unwrap();

    let mut mint = testutils::golden_mint(&module_path);
    let (_, sub_test_name) = testutils::current_test_name();
    let mut golden = mint.new_goldenfile(sub_test_name.unwrap()).unwrap();
    golden
        .write_all(got.as_bytes())
        .expect("Teardown: can't write to file to compare with golden");
}
