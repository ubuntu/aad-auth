use goldenfile::Mint;
use std::env;
use std::path::{Path, PathBuf};

// get_module_path returns the relative path to the module from the given file path.
pub fn get_module_path(path: &str) -> String {
    let mut path = PathBuf::from(path);
    path.pop();
    let path = path.to_str().unwrap();

    // Remove the base directory between the workspace and project
    let path = path.split('/').skip(1).collect::<Vec<&str>>().join("/");

    path
}

// current_test_name returns a tuple of (family_name, sub_test_name).
// Note that it can be called on tests without subtests too.
// The detection is based on thread name, and so, does not work when RUST_TEST_THREADS=1.
pub fn current_test_name() -> (String, Option<String>) {
    #[allow(clippy::or_fun_call)]
    if env::var("RUST_TEST_THREADS").unwrap_or("".to_string()) == "1" {
        panic!("Tests could not run with RUST_TEST_THREADS=1")
    }

    let cur_thread = std::thread::current();
    let parts: Vec<&str> = cur_thread.name().unwrap().split("::").collect();
    let family_name = parts[parts.len() - 2].to_string().to_lowercase();
    let subtest_name = parts[parts.len() - 1].to_string().to_lowercase();
    if subtest_name.starts_with("test_") {
        return (subtest_name, None);
    }
    (family_name, Some(subtest_name))
}

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
