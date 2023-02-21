use libnss::interop::Response;
use test_case::test_case;

use crate::cache::CacheError;

#[test_case(Ok(()), Response::Success(()); "Ok(T) is converted to Success(T)")]
#[test_case(Err(CacheError::DatabaseError("This is a database error".to_string())), Response::Unavail::<()>; "DatabaseError is converted to Unavail")]
#[test_case(Err(CacheError::QueryError("This is a query error".to_string())), Response::Unavail::<()>; "QueryError is converted to Unavail")]
#[test_case(Err(CacheError::NoRecord), Response::NotFound::<()>; "NoRecord is converted to Unavail")]
fn test_cache_result_to_nss_status<T>(r: Result<T, CacheError>, want_status: Response<T>)
where
    T: PartialEq,
{
    let got = super::cache_result_to_nss_status(r);
    assert!(
        got == want_status,
        "Expected {:?}, but got {:?}",
        want_status.to_status(),
        got.to_status()
    );
}
