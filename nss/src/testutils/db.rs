use log::debug;
use rusqlite::{self, Connection};
use std::{collections::HashMap, fs, path::Path};
use time;

#[derive(Debug)]
pub enum DBError {
    ConnError(String),
    CreationError(String),
    LoadDumpError(String),
    ParseDumpError(String),
}

const DB_NAMES: [&str; 2] = ["passwd", "shadow"];

/// Creates instances of the databases and initializes it with a inital state if requested.
pub fn prepare_db_for_tests(cache_dir: &Path, initial_state: Option<&str>) -> Result<(), DBError> {
    create_dbs_for_tests(&cache_dir)?;

    if let Some(state) = initial_state {
        let states_path = Path::new(&super::get_module_path(file!()))
            .join("states")
            .join(state);

        for db in DB_NAMES {
            let db_path = cache_dir.join(db.to_owned() + ".db");
            load_dump_into_db(&&states_path.join(format!("{db}.db.dump")), &db_path)?;
        }
    }

    Ok(())
}

/// Creates a database for testing purposes.
fn create_dbs_for_tests(cache_dir: &Path) -> Result<(), DBError> {
    debug!("Creating dabatase for tests");

    for db in DB_NAMES {
        let sql_path = Path::new(&super::get_module_path(file!()))
            .join("sql")
            .join(db.to_owned() + ".sql");

        let sql = match fs::read_to_string(&sql_path) {
            Ok(s) => s,
            Err(e) => return Err(DBError::CreationError(e.to_string())),
        };

        let conn = get_db_connection(&cache_dir.join(db.to_owned() + ".db"))?;

        if let Err(e) = conn.execute_batch(&sql) {
            return Err(DBError::CreationError(e.to_string()));
        }
    }

    Ok(())
}

/// Represents a database table.
#[derive(Debug)]
struct Table {
    /// Name of the database table.
    name: String,
    /// Name of the table columns.
    col_names: Vec<String>,
    /// 2D Vector with the table contents.
    rows: Vec<Vec<String>>,
}

/// Reads the content of the csv-like file located in the specified path
/// and parses it into a struct of Map<String, Table>.
fn read_dump_as_tables(dump_path: &Path) -> Result<HashMap<String, Table>, DBError> {
    let mut tables: HashMap<String, Table> = HashMap::new();

    let dump_file = match fs::read_to_string(dump_path) {
        Ok(content) => content,
        Err(e) => return Err(DBError::ParseDumpError(e.to_string())),
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
        for name in lines[1].split(",") {
            table.col_names.push(name.to_string());
        }

        // lines[2..] are the table rows
        for i in 2..lines.len() {
            let mut row = Vec::new();

            let tmp: Vec<&str> = lines[i].split(",").collect();
            for pos in 0..tmp.len() {
                row.push(tmp[pos].to_string());
            }

            table.rows.push(row);
        }
        tables.insert(table.name.clone(), table);
    }

    Ok(tables)
}

/// Reads a CSV dump file and loads its contents into the specified database.
fn load_dump_into_db(dump_path: &Path, db_path: &Path) -> Result<(), DBError> {
    debug!(
        "Loading passwd dump from {:?} into db",
        &dump_path.as_os_str()
    );

    let conn = get_db_connection(db_path)?;

    let tables = read_dump_as_tables(dump_path)?;
    for (name, table) in tables {
        let s = vec!["?,"; table.col_names.len()].concat();

        let stmt_str = format!("INSERT INTO {name} VALUES ({})", s.trim_end_matches(","));
        let mut stmt = match conn.prepare(&stmt_str) {
            Ok(stmt) => stmt,
            Err(e) => return Err(DBError::LoadDumpError(e.to_string())),
        };

        for row in table.rows {
            let mut values = row;
            for pos in 0..table.col_names.len() {
                if table.col_names[pos] == "last_online_auth" {
                    // Parses wildcards in order to load the correct time into the db.
                    values[pos] = parse_time_wildcard(&values[pos]).to_string();
                }
            }

            if let Err(e) = stmt.execute(rusqlite::params_from_iter(values)) {
                return Err(DBError::LoadDumpError(e.to_string()));
            };
        }
    }

    Ok(())
}

/// Parses some time wildcards that are contained in the dump files to ensure that
/// the loaded dbs will always present the same behavior when loaded for tests.
fn parse_time_wildcard(value: &str) -> i64 {
    // c is a contant value, set to two days, that is used to ensure that the time is within some intervals.
    let c = time::Duration::days(2);

    // TODO: Change after defining default expiration days in the cache module
    let expiration_days = time::Duration::days(90);

    let addend: time::Duration = match value {
        "RECENT_TIME" => -c,
        "PURGED_TIME" => (-2 * expiration_days) + c,
        "EXPIRED_TIME" => -expiration_days + c,
        "FUTURE_TIME" => c,
        _ => time::Duration::ZERO,
    };

    let now = time::OffsetDateTime::now_utc();
    let parsed_value = now + addend;

    parsed_value.unix_timestamp()
}

/// Retuns a connection to the database `db_path`.
fn get_db_connection(db_path: &Path) -> Result<Connection, DBError> {
    debug!("Connecting to db in {:?}", &db_path.as_os_str());

    // TODO: Fix permissions and checks after implementing shadow module.
    match Connection::open(&db_path) {
        Ok(conn) => Ok(conn),
        Err(e) => Err(DBError::ConnError(e.to_string())),
    }
}
