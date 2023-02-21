use rusqlite::{self, Connection};
use std::{
    boxed::Box,
    collections::HashMap,
    fs::{self, Permissions},
    os::unix::prelude::PermissionsExt,
    path::Path,
};
use tempfile::TempDir;
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
pub struct OptionalArgs {
    /// initial_state defines a path containing dump files to be loaded into the databases.
    pub initial_state: Option<String>,
    /// passwd_perms defines the unix permissions that will be set to the passwd database.
    pub passwd_perms: u32,
    /// shadow_perms defines the unix permissions that will be set to the shadow database.
    pub shadow_perms: u32,
}
impl Default for OptionalArgs {
    fn default() -> Self {
        Self {
            initial_state: None,
            passwd_perms: PASSWD_PERMS,
            shadow_perms: SHADOW_PERMS,
        }
    }
}

/// OptionalArgFn represent a function that overrides a default value in the OptionalArgs struct.
pub type OptionalArgFn = Box<dyn Fn(&mut OptionalArgs)>;

#[allow(dead_code)]
/// with_initial_state overrides the default initial state for the test database.
pub fn with_initial_state(state: Option<String>) -> OptionalArgFn {
    Box::new(move |o| o.initial_state = state.clone())
}

#[allow(dead_code)]
/// with_passwd_perms overrides the default passwd permissions for the test database.
pub fn with_passwd_perms(mode: u32) -> OptionalArgFn {
    Box::new(move |o| o.passwd_perms = mode)
}

#[allow(dead_code)]
/// with_shadow_perms overrides the default shadow permissions for the test database.
pub fn with_shadow_perms(mode: u32) -> OptionalArgFn {
    Box::new(move |o| o.shadow_perms = mode)
}

/// prepare_db_for_tests creates instances of the databases and initializes it based on the initial_state.
/// The following states are supported:
/// - None: Does not create the databases;
/// - Some(state): Creates the database(s) and load the contents from states/state/ into it.
pub fn prepare_db_for_tests(opts: Vec<OptionalArgFn>) -> Result<Option<TempDir>, Error> {
    init_logger();

    let mut args = OptionalArgs {
        ..Default::default()
    };
    for o in opts {
        o(&mut args);
    }

    if args.initial_state.is_none() {
        return Ok(None);
    }

    let state = args.initial_state.unwrap();
    let states_path = Path::new(&super::get_module_path(file!()))
        .join("states")
        .join(&state);

    let cache_dir = match TempDir::new() {
        Ok(dir) => dir,
        Err(err) => return Err(Error::Creation(err.to_string())),
    };
    let cache_path = cache_dir.path();

    for db in DB_NAMES {
        let dump_path = states_path.join(format!("{db}.db.dump"));
        if !dump_path.exists() {
            continue;
        }
        create_db_for_tests(cache_path, db)?;

        let db_path = cache_path.join(db.to_owned() + ".db");
        load_dump_into_db(&states_path.join(format!("{db}.db.dump")), &db_path)?;

        // Fix database permissions
        let db_path = cache_path.join(db.to_owned() + ".db");
        let perm = match db {
            "passwd" => args.passwd_perms,
            "shadow" => args.shadow_perms,
            _ => 0o000000,
        };

        if let Err(err) = fs::set_permissions(db_path, Permissions::from_mode(perm)) {
            return Err(Error::Creation(err.to_string()));
        }
    }

    Ok(Some(cache_dir))
}

/// create_db_for_tests creates a database for testing purposes.
fn create_db_for_tests(cache_dir: &Path, db: &str) -> Result<(), Error> {
    debug!("Creating dabatase for tests");

    let sql_path = Path::new(&super::get_module_path(file!()))
        .join("sql")
        .join(db.to_string() + ".sql");

    let sql = match fs::read_to_string(sql_path) {
        Ok(s) => s,
        Err(err) => return Err(Error::Creation(err.to_string())),
    };

    let conn = get_db_connection(&cache_dir.join(db.to_string() + ".db"))?;

    if let Err(err) = conn.execute_batch(&sql) {
        return Err(Error::Creation(err.to_string()));
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
