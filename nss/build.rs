fn main() {
    #[cfg(feature = "integration-tests")]
    cc::Build::new()
        .file("src/db_override.c")
        .define("INTEGRATION_TESTS", "1")
        .compile("db_override");
}
