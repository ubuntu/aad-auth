use std::fmt::Debug;

/// require_error checks if Result is Err and panics otherwise.
pub fn require_error<T, E>(got: Result<T, E>, prefix: &str)
where
    T: Debug,
{
    got.expect_err(&format!(
        "{prefix} should have returned an error, but didn't"
    ));
}

/// require_no_error checks if Result is Ok and panics otherwise.
pub fn require_no_error<T, E>(got: Result<T, E>, prefix: &str)
where
    E: Debug,
{
    got.unwrap_or_else(|err| {
        panic!("{prefix} should not have returned an error, but did: {err:?}")
    });
}
