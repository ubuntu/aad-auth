use super::current_test_name;
use goldenfile::Mint;
use std::path::Path;

// golden_mint returns the golden Mint based on the family test name of current test.
// If there is no subtests, then, it returns the parent directory based on test name
// where you will add a "golden"
// file.
// The detection is based on thread name, and so, does not work when RUST_TEST_THREADS=1.
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
