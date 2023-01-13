use std::path::PathBuf;
use std::{env, fs, io::Write};

use core::panic;
use goldenfile::Mint;
use serde_yaml::to_string;
use tempdir::TempDir;
use test_case::test_case;

use crate::CacheDB;

// get_current_module_path returns the path to the current module.
fn get_current_module_path() -> String {
    let mut path = PathBuf::from(file!());
    path.pop();
    let path = path.to_str().unwrap();

    // Remove the base directory between the workspace and project
    let path = path.split('/').skip(1).collect::<Vec<&str>>().join("/");

    path + "/"
}

// current_test_name returns a tuple of (parent_test_name, sub_test_name).
// The detection is based on thread name, and so, does not work when RUST_TEST_THREADS=1
fn current_test_name() -> (String, String) {
    #[allow(clippy::or_fun_call)]
    if env::var("RUST_TEST_THREADS").unwrap_or("".to_string()) == "1" {
        panic!("Tests could not run with RUST_TEST_THREADS=1")
    }

    let cur_thread = std::thread::current();
    let parts: Vec<&str> = cur_thread.name().unwrap().split("::").collect();
    (
        parts[parts.len() - 2].to_string(),
        parts[parts.len() - 1].to_string(),
    )
}

#[test_case(2408865428, false  ; "get existing user")]
#[test_case(4242, true  ; "error on non existing user")]
fn test_get_passwd_from_uid(uid: u32, want_err: bool) {
    let test_name = current_test_name();

    let cache_dir =
        TempDir::new("test-aad-auth").expect("Setup: could not create temporary cache directory");

    let passwd_db = cache_dir.path().join("passwd.db");

    // TODO: unmarshall dbs from cache_dumps
    fs::copy(
        get_current_module_path() + "../../../cache/passwd.db",
        passwd_db,
    )
    .expect("Setup: could not copy existing database");

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
    let mut mint = Mint::new(get_current_module_path() + "testdata/golden/" + &test_name.0);
    let mut golden = mint.new_goldenfile(test_name.1).unwrap();
    golden
        .write_all(got.as_bytes())
        .expect("Teardown: can't write to file to compare with golden");
}
