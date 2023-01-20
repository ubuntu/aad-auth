use std::path::Path;

use rusqlite::{Connection, OpenFlags, Rows, Statement};
use serde::Serialize;

use log::debug;

#[cfg(test)]
mod mod_tests;

const DB_PATH: &str = "/var/lib/aad/cache";

/// Passwd struct represents a password entry in the cache database.
#[derive(Debug, Serialize)]
pub struct Passwd {
    pub name: String,
    pub passwd: String,
    pub uid: u32,
    pub gid: u32,
    pub gecos: String,
    pub home: String,
    pub shell: String,
}

/// CacheError enum represents the list of errors supported by the cache.
#[derive(Debug)]
pub enum CacheError {
    DatabaseError(String),
    QueryError(String),
    NoRecord,
}

/// CacheDB struct represents the cache database.
pub struct CacheDB {
    conn: Connection,
}

/// CacheDBBuilder struct is the struct for the builder pattern and change the parameters of the cache.
pub struct CacheDBBuilder {
    db_path: Option<String>,
}

impl CacheDBBuilder {
    /// with_db_path overrides the path to the cache database.
    pub fn with_db_path(&mut self, db_path: &str) -> &mut Self {
        debug!("using custom db path: {}", db_path);
        self.db_path = Some(db_path.to_string());
        self
    }

    /// build initializes and opens a connection to the cache database.
    pub fn build(&self) -> Result<CacheDB, CacheError> {
        let mut db_path = DB_PATH.to_string();
        if let Some(path_override) = &self.db_path {
            db_path = path_override.to_string();
        }

        let passwd_db = Path::new(&db_path).join("passwd.db");
        let passwd_db = passwd_db.to_str().unwrap();

        debug!("opening database connection from {}", passwd_db);

        let conn = match Connection::open_with_flags(passwd_db, OpenFlags::SQLITE_OPEN_READ_ONLY) {
            Ok(conn) => conn,
            Err(err) => return Err(CacheError::DatabaseError(err.to_string())),
        };

        // TODO: attach shadow if root. Handle file permissions…

        Ok(CacheDB { conn })
    }
}

impl CacheDB {
    /// new creates a new CacheDBBuilder object.
    #[allow(clippy::new_ret_no_self)] // builder pattern
    pub fn new() -> CacheDBBuilder {
        CacheDBBuilder { db_path: None }
    }

    /* Passwd */
    /// get_passwd_by_uid queries the database for a passwd row with matching uid.
    pub fn get_passwd_by_uid(&self, uid: u32) -> Result<Passwd, CacheError> {
        let mut stmt = self.prepare_statement(
            "SELECT login, password, uid, gid, gecos, home, shell FROM passwd WHERE uid = ?", // Last empty field is the shadow password
        )?;

        let rows = match stmt.query([uid]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        let mut entries = Self::rows_to_passwd_entries(rows);

        Self::expect_one_row(&mut entries)
    }

    /// get_passwd_by_name queries the database for a passwd row with matching name.
    pub fn get_passwd_by_name(&self, login: &str) -> Result<Passwd, CacheError> {
        let mut stmt = self.prepare_statement(
            "SELECT login, password, uid, gid, gecos, home, shell FROM passwd WHERE login = ?", // Last empty field is the shadow password
        )?;

        let rows = match stmt.query([login]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        let mut entries = Self::rows_to_passwd_entries(rows);

        Self::expect_one_row(&mut entries)
    }

    /// get_all_passwd queries the database for all passwd rows.
    pub fn get_all_passwd(&self) -> Result<Vec<Passwd>, CacheError> {
        let mut stmt = self.prepare_statement(
            "SELECT login, password, uid, gid, gecos, home, shell FROM passwd", // Last empty field is the shadow password
        )?;

        let rows = match stmt.query([]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        Ok(Self::rows_to_passwd_entries(rows))
    }

    /* Common */
    /// prepare_statement prepares a statement and queries the database.
    fn prepare_statement(&self, stmt_str: &str) -> Result<Statement, CacheError> {
        match self.conn.prepare(stmt_str) {
            Ok(stmt) => Ok(stmt),
            Err(err) => Err(CacheError::QueryError(err.to_string())),
        }
    }

    /// rows_to_passwd_entries converts SQL rows to `Vec<Passwd>`.
    fn rows_to_passwd_entries(mut rows: Rows) -> Vec<Passwd> {
        let mut entries = Vec::new();
        while let Ok(Some(row)) = rows.next() {
            entries.push(Passwd {
                name: row.get(0).expect("invalid name"),
                passwd: row.get(1).expect("invalid passwd"),
                uid: row.get(2).expect("invalid uid"),
                gid: row.get(3).expect("invalid gid"),
                gecos: row.get(4).expect("invalid gecos"),
                home: row.get(5).expect("invalid home"),
                shell: row.get(6).expect("invalid shell"),
            });
        }
        entries
    }

    /// expect_one_row returns an error if `entries` contains no rows or more than one row.
    fn expect_one_row<T>(entries: &mut Vec<T>) -> Result<T, CacheError> {
        if entries.len() > 1 {
            return Err(CacheError::DatabaseError(
                "More than one entry found".to_string(),
            ));
        }

        match entries.pop() {
            Some(entry) => Ok(entry),
            None => Err(CacheError::NoRecord),
        }
    }
}
