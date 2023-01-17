use std::io::Write;

use serde_yaml::to_string;
use tempdir::TempDir;
use test_case::test_case;

use crate::testutils;
use crate::CacheDB;

#[test_case(165119649, false  ; "Get existing user")]
#[test_case(4242, true  ; "Error on non existing user")]
fn test_get_passwd_from_uid(uid: u32, want_err: bool) {
    let module_path = testutils::get_module_path(file!());

    let cache_dir =
        TempDir::new("test-aad-auth").expect("Setup: could not create temporary cache directory");

    let passwd_db = cache_dir.path().join("passwd.db");

    if let Some(passwd_str_path) = passwd_db.to_str() {
        let state = "users_in_db";
        if let Err(e) = testutils::prepare_db_for_tests(passwd_str_path, Some(&state)) {
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
