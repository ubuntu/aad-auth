use crate::{cache::Passwd, testutils::get_module_path};
use log::debug;
use rusqlite::Connection;
use std::{fs::read_to_string, path::Path};

#[derive(Debug)]
pub enum DBError {
    ConnError(String),
    CreationError(String),
    LoadDumpError(String),
    SaveDumpError(String),
}

// prepare_db_for_tests creates an instance of the database and initializes it with a inital state
// if requested.
pub fn prepare_db_for_tests(db_path: &str, initial_state: Option<&str>) -> Result<(), DBError> {
    create_db_for_tests(db_path)?;

    if let Some(state) = initial_state {
        let states_path = Path::new(&get_module_path(file!()))
            .join("states")
            .join(state);

        load_passwd_dump_into_db(db_path, &states_path.join("passwd.dump"))?;
    }

    Ok(())
}

// create_db_for_tests creates a database for testing purposes.
fn create_db_for_tests(db_path: &str) -> Result<(), DBError> {
    debug!("Creating dabatase for tests");

    if Path::new(&db_path).exists() {
        return Ok(());
    }

    let conn = get_db_connection(db_path)?;

    for db_name in ["passwd", "shadow"] {
        let sql_path = get_module_path(file!()) + "/sql/" + db_name + ".sql";

        let sql = match read_to_string(Path::new(&sql_path)) {
            Ok(s) => s,
            Err(e) => return Err(DBError::CreationError(e.to_string())),
        };

        if let Err(e) = conn.execute_batch(&sql) {
            return Err(DBError::CreationError(e.to_string()));
        }
    }

    Ok(())
}

// load_passwd_dump_into_db reads a CSV dump file and loads its contents into the passwd table.
fn load_passwd_dump_into_db(db_path: &str, dump_path: &Path) -> Result<(), DBError> {
    debug!(
        "Loading passwd dump from {:?} into db",
        &dump_path.as_os_str()
    );

    let conn = get_db_connection(db_path)?;

    let mut reader = match csv::Reader::from_path(dump_path) {
        Ok(r) => r,
        Err(e) => return Err(DBError::LoadDumpError(e.to_string())),
    };

    let mut stmt = match conn.prepare("INSERT INTO passwd VALUES (?, ?, ?, ?, ?, ?, ?, ?)") {
        Ok(stmt) => stmt,
        Err(e) => return Err(DBError::LoadDumpError(e.to_string())),
    };

    for item in reader.deserialize() {
        let record: Passwd = match item {
            Ok(r) => r,
            Err(e) => return Err(DBError::LoadDumpError(e.to_string())),
        };

        if let Err(e) = stmt.execute((
            record.name,
            record.passwd,
            record.uid,
            record.gid,
            record.gecos,
            record.home,
            record.shell,
            "0",
        )) {
            return Err(DBError::LoadDumpError(e.to_string()));
        }
    }

    Ok(())
}

// dump_passwd_db queries the passwd table in the db and dumps its contents into the specified path.
pub fn dump_passwd_db(db_path: &str, dump_path: &Path) -> Result<(), DBError> {
    debug!("Dumping passwd table to {:?}", &dump_path.as_os_str());

    let conn = get_db_connection(db_path)?;

    let mut stmt = match conn.prepare("SELECT * FROM passwd") {
        Ok(stmt) => stmt,
        Err(e) => return Err(DBError::SaveDumpError(e.to_string())),
    };

    let mut rows = match stmt.query([]) {
        Ok(rows) => rows,
        Err(e) => return Err(DBError::SaveDumpError(e.to_string())),
    };

    let mut dump_writer = match csv::Writer::from_path(dump_path) {
        Ok(writer) => writer,
        Err(e) => return Err(DBError::SaveDumpError(e.to_string())),
    };

    while let Ok(Some(row)) = rows.next() {
        let entry = Passwd {
            name: row.get(0).expect("invalid name"),
            passwd: row.get(1).expect("invalid passwd"),
            uid: row.get(2).expect("invalid uid"),
            gid: row.get(3).expect("invalid gid"),
            gecos: row.get(4).expect("invalid gecos"),
            home: row.get(5).expect("invalid home"),
            shell: row.get(6).expect("invalid shell"),
        };

        if let Err(e) = dump_writer.serialize(&entry) {
            return Err(DBError::SaveDumpError(e.to_string()));
        }
    }

    match dump_writer.flush() {
        Ok(_) => Ok(()),
        Err(e) => Err(DBError::SaveDumpError(e.to_string())),
    }
}

// get_db_connection returns a connection to the specified database.
fn get_db_connection(db_path: &str) -> Result<Connection, DBError> {
    debug!("Connecting to db in {}", &db_path);

    match Connection::open(db_path) {
        Ok(conn) => Ok(conn),
        Err(e) => Err(DBError::ConnError(e.to_string())),
    }
}
