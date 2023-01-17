use std::io::Write;

use serde_yaml::to_string;
use tempfile::TempDir;
use test_case::test_case;

use crate::testutils;
use crate::CacheDB;

#[test_case(165119649, Some("users_in_db"), false  ; "Get existing user")]
#[test_case(4242, Some("users_in_db"), true  ; "Error on non existing user")]
fn test_get_passwd_from_uid(uid: u32, initial_state: Option<&str>, want_err: bool) {
    let module_path = testutils::get_module_path(file!());

    let cache_dir = TempDir::new().expect("Setup: could not create temporary cache directory");

    if let Some(cache_str_path) = cache_dir.path().to_str() {
        if let Err(e) = testutils::prepare_db_for_tests(cache_str_path, initial_state) {
            panic!("Setup: Failed to prepare db for tests: {:?}", e);
        }
    }

    let c = CacheDB::new()
        .with_db_path(cache_dir.path().to_str().unwrap())
        .build()
        .expect("Setup: could not create cache object");

    let got = c.get_passwd_from_uid(uid);
    if let Err(e) = got {
        assert!(
            want_err,
            "get_passwd_from_uid should not have returned an error but did: {:?}",
            e,
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
