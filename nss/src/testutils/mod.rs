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

// current_test_name returns a tuple of (parent_test_name, sub_test_name).
// The detection is based on thread name, and so, does not work when RUST_TEST_THREADS=1.
pub fn current_test_name() -> (String, String) {
    #[allow(clippy::or_fun_call)]
    if env::var("RUST_TEST_THREADS").unwrap_or("".to_string()) == "1" {
        panic!("Tests could not run with RUST_TEST_THREADS=1")
    }

    let cur_thread = std::thread::current();
    let parts: Vec<&str> = cur_thread.name().unwrap().split("::").collect();
    (
        parts[parts.len() - 2].to_string(),
        parts[parts.len() - 1].to_string().to_lowercase(),
    )
}

// golden_mint returns the golden Mint based on the family test name of current test.
// The detection is based on thread name, and so, does not work when RUST_TEST_THREADS=1.
pub fn golden_mint(module_path: &str) -> Mint {
    let (family_name, _) = current_test_name();
    Mint::new(
        Path::new(module_path)
            .join("testdata")
            .join(&family_name)
            .join("golden"),
    )
}
