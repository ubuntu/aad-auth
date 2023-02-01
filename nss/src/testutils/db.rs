use rusqlite::{self, Connection};
use std::{
    collections::HashMap,
    ffi::CString,
    fs::{self, Permissions},
    os::unix::prelude::PermissionsExt,
    path::Path,
};
use time::{Duration, OffsetDateTime};

use crate::{
    cache::{
        EXPIRATION_PURGE_MULTIPLIER, OFFLINE_CREDENTIALS_EXPIRATION, PASSWD_PERMS, SHADOW_PERMS,
    },
    debug, init_logger,
};

/// Error enum represents the error codes that can be returned by this module.
#[derive(Debug)]
pub enum Error {
    Connection(String),
    Creation(String),
    LoadDump(String),
    ParseDump(String),
}

const DB_NAMES: [&str; 2] = ["passwd", "shadow"];

/// OptionalArgs represents optional arguments that can be provided to prepare_db_for_tests.
#[derive(Default)]
pub struct OptionalArgs<'a> {
    /// initial_state defines a path containing dump files to be loaded into the databases.
    pub initial_state: Option<&'a str>,
    /// root_uid defines the uid to be used as root_uid when setting the database file permissions.
    /// If None, the current user id is used instead.
    pub root_uid: Option<u32>,
    /// root_gid defines the gid to be used as root_gid when setting the database file permissions.
    /// If None, the current user gid is used instead.
    pub root_gid: Option<u32>,
    /// shadow_gid defines the gid to be used as shadow_gid when setting the database file permissions.
    /// If None, the current user gid is used instead.
    pub shadow_gid: Option<u32>,
}

/// prepare_db_for_tests creates instances of the databases and initializes it with a inital state if requested.
pub fn prepare_db_for_tests(cache_dir: &Path, opts: OptionalArgs) -> Result<(), Error> {
    init_logger();
    create_dbs_for_tests(cache_dir)?;

    // Loads saved state into the database.
    if let Some(state) = opts.initial_state {
        let states_path = Path::new(&super::get_module_path(file!()))
            .join("states")
            .join(state);

        for db in DB_NAMES {
            let db_path = cache_dir.join(db.to_owned() + ".db");
            load_dump_into_db(&states_path.join(format!("{db}.db.dump")), &db_path)?;
        }
    }

    // Fix database permissions
    for db in DB_NAMES {
        // TODO: Investigate using Go-like optional arguments
        let (mut uid, mut gid) = (users::get_current_uid(), users::get_current_gid());
        let mut shadow_gid = users::get_group_by_name("shadow").unwrap().gid();
        if let Some(id) = opts.root_uid {
            uid = id;
        }
        if let Some(id) = opts.root_gid {
            gid = id;
        }
        if let Some(id) = opts.shadow_gid {
            shadow_gid = id;
        }

        let db_path = cache_dir.join(db.to_owned() + ".db");
        let (perm, gid) = match db {
            "passwd" => (PASSWD_PERMS, gid),
            "shadow" => (SHADOW_PERMS, shadow_gid),
            _ => (0o000000, gid),
        };

        // libc functions are unstable and, thus, require a unsafe block.
        unsafe {
            // Sets the correct ownership for the db files after loading the dumps.
            // Need to use unsafe libc function while https://doc.rust-lang.org/std/os/unix/fs/fn.chown.html
            // is still in nightly.
            let c_path = CString::new(db_path.to_str().unwrap()).unwrap();
            match libc::chown(c_path.as_ptr(), uid, gid) {
                ok if ok <= 0 => (),
                err if err > 0 => {
                    return Err(Error::Creation(format!(
                        "failed to change ownership of {db_path:?}: {err}"
                    )))
                }
                _ => (),
            };
        }

        if let Err(err) = fs::set_permissions(db_path, Permissions::from_mode(perm)) {
            return Err(Error::Creation(err.to_string()));
        }
    }

    Ok(())
}

/// create_dbs_for_tests creates a database for testing purposes.
fn create_dbs_for_tests(cache_dir: &Path) -> Result<(), Error> {
    debug!("Creating dabatase for tests");

    for db in DB_NAMES {
        let sql_path = Path::new(&super::get_module_path(file!()))
            .join("sql")
            .join(db.to_owned() + ".sql");

        let sql = match fs::read_to_string(sql_path) {
            Ok(s) => s,
            Err(err) => return Err(Error::Creation(err.to_string())),
        };

        let conn = get_db_connection(&cache_dir.join(db.to_owned() + ".db"))?;

        if let Err(err) = conn.execute_batch(&sql) {
            return Err(Error::Creation(err.to_string()));
        }
    }

    Ok(())
}

/// Table struct represents a database table.
#[derive(Debug)]
struct Table {
    /// Name of the database table.
    name: String,
    /// Names of the table columns.
    col_names: Vec<String>,
    /// Vector with the table contents.
    rows: Vec<Vec<String>>,
}

/// read_dump_as_tables reads the content of the csv-like file located in the
/// specified path and parses it into a struct of Map<String, Table>.
fn read_dump_as_tables(dump_path: &Path) -> Result<HashMap<String, Table>, Error> {
    let mut tables: HashMap<String, Table> = HashMap::new();

    let dump_file = match fs::read_to_string(dump_path) {
        Ok(content) => content,
        Err(err) => return Err(Error::ParseDump(err.to_string())),
    };

    let data: Vec<&str> = dump_file.split_terminator("\n\n").collect();
    for table in data {
        let lines: Vec<&str> = table.lines().collect();

        let mut table = Table {
            name: String::default(),
            col_names: Vec::new(),
            rows: Vec::new(),
        };

        // lines[0] is the table name
        table.name = lines[0].to_string();

        // lines[1] is the columns names
        for name in lines[1].split(',') {
            table.col_names.push(name.to_string());
        }

        // lines[2..] are the table rows
        for line in lines.iter().skip(2) {
            let mut row = Vec::new();

            let values: Vec<&str> = line.split(',').collect();
            for value in values.iter() {
                row.push(value.to_string());
            }

            table.rows.push(row);
        }
        tables.insert(table.name.clone(), table);
    }

    Ok(tables)
}

/// load_dump_into_db reads a CSV dump file and loads its contents into the specified database.
fn load_dump_into_db(dump_path: &Path, db_path: &Path) -> Result<(), Error> {
    debug!(
        "Loading passwd dump from {:?} into db",
        &dump_path.as_os_str()
    );

    let conn = get_db_connection(db_path)?;

    let tables = read_dump_as_tables(dump_path)?;
    for (name, table) in tables {
        let s = vec!["?,"; table.col_names.len()].concat();

        let stmt_str = format!("INSERT INTO {name} VALUES ({})", s.trim_end_matches(','));
        let mut stmt = match conn.prepare(&stmt_str) {
            Ok(stmt) => stmt,
            Err(err) => return Err(Error::LoadDump(err.to_string())),
        };

        for row in table.rows {
            let mut values = row;

            // Handling special cases for some columns.
            for (pos, col_name) in table.col_names.iter().enumerate() {
                if col_name == "last_online_auth" {
                    values[pos] = parse_time_wildcard(&values[pos]).to_string();
                }
            }

            if let Err(err) = stmt.execute(rusqlite::params_from_iter(values)) {
                return Err(Error::LoadDump(err.to_string()));
            };
        }
    }

    Ok(())
}

/// parse_time_wildcard parses some time wildcards that are contained in the dump files
/// to ensure that the loaded dbs will always present the same behavior when loaded for tests.
fn parse_time_wildcard(value: &str) -> i64 {
    let expiration_days = Duration::days(OFFLINE_CREDENTIALS_EXPIRATION.into());

    // c is a contant value, set to two days, that is used to ensure that the time is within some intervals.
    let c = Duration::days(2);
    let addend: Duration = match value {
        "RECENT_TIME" => -1 * c,
        "EXPIRED_TIME" => -1 * (expiration_days + c),
        "PURGED_TIME" => -1 * ((EXPIRATION_PURGE_MULTIPLIER * expiration_days) + c),
        "FUTURE_TIME" => c,
        _ => Duration::ZERO,
    };

    let parsed_value = OffsetDateTime::now_utc() + addend;
    parsed_value.unix_timestamp()
}

/// get_db_connection retuns a connection to the database `db_path`.
fn get_db_connection(db_path: &Path) -> Result<Connection, Error> {
    debug!("Connecting to db in {:?}", &db_path.as_os_str());

    match Connection::open(db_path) {
        Ok(conn) => Ok(conn),
        Err(err) => Err(Error::Connection(err.to_string())),
    }
}
