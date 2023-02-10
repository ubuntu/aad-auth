use log::{LevelFilter, Metadata};
use simple_logger::SimpleLogger;
use std::env;
use syslog::{BasicLogger, Facility, Formatter3164};

#[macro_export]
macro_rules! debug {
    ($($arg:tt)*) => {
        let log_prefix = "nss_aad:";
        log::debug!("{} {}", log_prefix, format_args!($($arg)*));
    }
}

#[macro_export]
macro_rules! error {
    ($($arg:tt)*) => {
        let log_prefix = "nss_aad:";
        log::error!("{} {}", log_prefix, format_args!($($arg)*));
    }
}

/// init_logger initialize the global logger with a default level set to info. This function is only
/// required to be called once and is a no-op on subsequent calls.
///
/// The log level can be set to debug by setting the environment variable NSS_AAD_DEBUG.
pub fn init_logger() {
    if log::logger().enabled(&Metadata::builder().build()) {
        return;
    }

    let mut level = LevelFilter::Info;
    if let Ok(target) = env::var("NSS_AAD_DEBUG") {
        level = LevelFilter::Debug;
        match target {
            s if s == *"stderr" => init_stderr_logger(level),
            _ => init_sys_logger(level),
        }
    } else {
        init_sys_logger(level);
    }

    debug!("Log level set to {:?}", level);
}

/// init_sys_logger initializes a global log that prints messages to the system logs.
fn init_sys_logger(log_level: LevelFilter) {
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

    if let Err(err) = log::set_boxed_logger(Box::new(BasicLogger::new(logger)))
        .map(|()| log::set_max_level(log_level))
    {
        eprintln!("cannot set log level: {err:?}");
        return;
    };

    debug!("Log output set to syslog");
}

/// init_stderr_logger initializes a global log that prints the messages to stderr.
fn init_stderr_logger(log_level: LevelFilter) {
    SimpleLogger::new().with_level(log_level).init().unwrap();
    debug!("Log output set to stderr");
}
