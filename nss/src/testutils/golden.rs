use super::current_test_name;
use goldenfile::Mint;
use serde::Serialize;
use std::{io::Write, path::Path};

/// golden_mint returns the golden Mint based on the family test name of current test.
/// If there is no subtests, then, it returns the parent directory based on test name
/// where you will add a "golden" file.
///
/// The detection is based on thread name, and so, does not work when RUST_TEST_THREADS=1.
pub fn golden_mint(module_path: &str) -> Mint {
    let (family_name, subtest_name) = current_test_name();

    match subtest_name {
        // If there is a subtest, "golden" is then the Mint director.
        Some(_) => Mint::new(
            Path::new(module_path)
                .join("testdata")
                .join(&family_name)
                .join("golden"),
        ),
        // If there is none, then the directory name is the family name and golden is the file.
        None => Mint::new(Path::new(module_path).join("testdata").join(&family_name)),
    }
}

/// load_and_update_golden compares the specified got with its golden file and regenerates the file
/// with got content if the env REGENERATE_GOLDENFILES is set.
pub fn load_and_update_golden<S>(module_path: &str, got: S)
where
    S: Serialize,
{
    let serialized_got = serde_yaml::to_string(&got).unwrap();

    let (_, sub_test_name) = super::current_test_name();
    let sub_test_name = sub_test_name.unwrap();

    let mut mint = golden_mint(module_path);
    let mut golden = mint.new_goldenfile(&sub_test_name).unwrap();

    match golden.write_all(serialized_got.as_bytes()) {
        Ok(()) => (),
        Err(err) => panic!("Teardown: failed to write golden file for {sub_test_name}: {err}"),
    };
}
