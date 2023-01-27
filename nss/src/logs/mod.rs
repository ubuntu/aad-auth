use std::env;

use ctor::ctor;
use syslog::{BasicLogger, Facility, Formatter3164};

pub const LOGPREFIX: &str = "nss_aad:";

#[macro_export]
macro_rules! debug {
    ($($arg:tt)*) => {
        log::debug!("{} {}", LOGPREFIX, format_args!($($arg)*));
    }
}

#[macro_export]
macro_rules! error {
    ($($arg:tt)*) => {
        log::error!("{} {}", LOGPREFIX, format_args!($($arg)*));
    }
}

// init_logger initialize the logger with a default level set to info
// log level can be set to debug by setting the environment variable NSS_AAD_DEBUG
#[ctor]
fn init_logger() {
    let formatter = Formatter3164 {
        facility: Facility::LOG_USER,
        hostname: None,
        process: "aad_auth".into(),
        pid: 0,
    };

    let logger = match syslog::unix(formatter) {
        Err(err) => {
            println!("cannot connect to syslog: {err:?}");
            return;
        }
        Ok(l) => l,
    };

    let mut level = log::LevelFilter::Info;
    let show_debug = env::var("NSS_AAD_DEBUG").unwrap_or_default();
    if !show_debug.is_empty() {
        level = log::LevelFilter::Debug;
    }
    if let Err(err) = log::set_boxed_logger(Box::new(BasicLogger::new(logger)))
        .map(|()| log::set_max_level(level))
    {
        eprintln!("cannot set log level: {err:?}");
    };

    crate::debug!("Log level set to {:?}", level);
}
