use ctor::ctor;
use std::env;
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

#[ctor]
fn init_logger() {
    let formatter = Formatter3164 {
        facility: Facility::LOG_USER,
        hostname: None,
        process: "aad_auth".into(),
        pid: 0,
    };

    let logger = match syslog::unix(formatter) {
        Err(e) => {
            println!("cannot connect to syslog: {:?}", e);
            return;
        }
        Ok(l) => l,
    };

    let mut level = log::LevelFilter::Info;
    let show_debug = env::var("NSS_AAD_DEBUG").unwrap_or("".to_string());
    if show_debug != "" {
        level = log::LevelFilter::Debug;
    }
    if let Err(e) = log::set_boxed_logger(Box::new(BasicLogger::new(logger)))
        .map(|()| log::set_max_level(level))
    {
        eprintln!("cannot set log level: {:?}", e);
    };

    crate::debug!("Log level set to {:?}", level);
}
