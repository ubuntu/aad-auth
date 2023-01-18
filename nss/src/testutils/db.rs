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

// prepare_db_for_tests creates instances of the databases and initializes it with a inital state
// if requested.
pub fn prepare_db_for_tests(db_path: &str, initial_state: Option<&str>) -> Result<(), DBError> {
    create_dbs_for_tests(&db_path)?;

    if let Some(state) = initial_state {
        let states_path = Path::new(&super::get_module_path(file!()))
            .join("states")
            .join(state);

        for db in ["passwd", "shadow"] {
            load_dump_into_db(&db_path, &db, &states_path.join(format!("{db}.db.dump")))?;
        }
    }

    Ok(())
}

// create_db_for_tests creates a database for testing purposes.
fn create_dbs_for_tests(db_path: &str) -> Result<(), DBError> {
    debug!("Creating dabatase for tests");

    for db in ["passwd", "shadow"] {
        let sql_path = Path::new(&super::get_module_path(file!()))
            .join("sql")
            .join(db.to_owned() + ".sql");

        let sql = match fs::read_to_string(&sql_path) {
            Ok(s) => s,
            Err(e) => return Err(DBError::CreationError(e.to_string())),
        };

        let conn = match db {
            "passwd" => get_passwd_connection(db_path),
            "shadow" => get_shadow_connection(db_path),
            _ => Err(DBError::ConnError(String::new())),
        }?;

        if let Err(e) = conn.execute_batch(&sql) {
            return Err(DBError::CreationError(e.to_string()));
        }
    }

    Ok(())
}

#[derive(Debug)]
struct Table {
    name: String,
    col_names: Vec<String>,
    rows: Vec<HashMap<String, String>>,
}

fn read_dump_as_tables(dump_path: &Path) -> Result<HashMap<String, Table>, DBError> {
    let mut tables: HashMap<String, Table> = HashMap::new();

    let dump_file = match fs::read_to_string(dump_path) {
        Ok(content) => content,
        Err(e) => return Err(DBError::ParseDumpError(e.to_string())),
    };

    let data: Vec<&str> = dump_file.split_terminator("\n\n").collect();
    for table in data {
        let lines: Vec<&str> = table.lines().collect();
        if lines.len() == 0 {
            break;
        }

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
            let mut row: HashMap<String, String> = HashMap::new();

            let tmp: Vec<&str> = lines[i].split(",").collect();
            for pos in 0..tmp.len() {
                row.insert(table.col_names[pos].clone(), tmp[pos].to_string());
            }

            table.rows.push(row);
        }
        tables.insert(table.name.clone(), table);
    }

    Ok(tables)
}

// load_dump_into_db reads a CSV dump file and loads its contents into the specified database.
fn load_dump_into_db(db_path: &str, db_name: &str, dump_path: &Path) -> Result<(), DBError> {
    debug!(
        "Loading passwd dump from {:?} into db",
        &dump_path.as_os_str()
    );

    let conn = match db_name {
        "passwd" => get_passwd_connection(db_path),
        "shadow" => get_shadow_connection(db_path),
        _ => Err(DBError::ConnError(String::new())),
    }?;

    let tables = read_dump_as_tables(dump_path)?;
    for (name, table) in tables {
        let s = vec!["?,"; table.col_names.len()].concat();

        let stmt_str = format!("INSERT INTO {name} VALUES ({})", s.trim_end_matches(","));
        let mut stmt = match conn.prepare(&stmt_str) {
            Ok(stmt) => stmt,
            Err(e) => return Err(DBError::LoadDumpError(e.to_string())),
        };

        for row in table.rows {
            let mut values: Vec<String> = Vec::new();

            for name in table.col_names.iter() {
                if *name == "last_online_auth" {
                    // Parses wildcards in order to load the correct time into the db.
                    values.push(parse_time_wildcard(&row[name]).to_string());
                    continue;
                }
                values.push(row[name].to_string());
            }

            if let Err(e) = stmt.execute(rusqlite::params_from_iter(values)) {
                return Err(DBError::LoadDumpError(e.to_string()));
            };
        }
    }

    Ok(())
}

// parse_time_wildcard parses some time wildcards that are contained in the dump files to ensure that
// the loaded dbs will always present the same behavior when loaded for tests.
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

// get_passwd_connection returns a connection to the passwd database located in the specified path.
fn get_passwd_connection(db_path: &str) -> Result<Connection, DBError> {
    debug!("Connecting to passwd.db in {}", &db_path);

    match Connection::open(&Path::new(db_path).join("passwd.db")) {
        Ok(conn) => Ok(conn),
        Err(e) => Err(DBError::ConnError(e.to_string())),
    }
}

// get_shadow_connection returns a connection to the shadow database located in the specified path.
fn get_shadow_connection(db_path: &str) -> Result<Connection, DBError> {
    debug!("Connecting to shadow.db in {}", &db_path);

    // TODO: Fix permissions and checks after implementing shadow module.
    match Connection::open(&Path::new(db_path).join("shadow.db")) {
        Ok(conn) => Ok(conn),
        Err(e) => Err(DBError::ConnError(e.to_string())),
    }
}
