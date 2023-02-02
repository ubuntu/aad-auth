use std::fmt::Debug;

/// require_error checks if Result is Err and panics otherwise.
pub fn require_error<T, E>(got: Result<T, E>)
where
    T: Debug,
{
    let (test, subtest) = super::current_test_name();
    let mut test_name = test;
    if let Some(name) = subtest {
        test_name.push_str(&("/".to_owned() + &name));
    }

    got.expect_err(&format!(
        "{test_name} should have returned an error, but didn't"
    ));
}

/// require_no_error checks if Result is Ok and panics otherwise.
pub fn require_no_error<T, E>(got: Result<T, E>)
where
    E: Debug,
{
    let (test, subtest) = super::current_test_name();
    let mut test_name = test;
    if let Some(name) = subtest {
        test_name.push_str(&("/".to_owned() + &name));
    }

    got.unwrap_or_else(|err| {
        panic!("{test_name} should not have returned an error, but did: {err:?}")
    });
}
