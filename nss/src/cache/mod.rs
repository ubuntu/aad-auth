use std::{
    fs::{self, Permissions},
    os::unix::fs::MetadataExt,
    os::unix::fs::PermissionsExt,
    path::{Path, PathBuf},
};

use rusqlite::{Connection, OpenFlags, Rows, Statement};
use serde::Serialize;

use log::debug;

#[cfg(test)]
mod mod_tests;

const DB_PATH: &str = "/var/lib/aad/cache";
const PASSWD_DB: &str = "passwd.db"; // root:root 644
const SHADOW_DB: &str = "shadow.db"; // root:shadow 640

/// ShadowMode enum represents the status of the shadow database.
#[derive(PartialEq, PartialOrd, Debug, Clone, Copy)]
pub enum ShadowMode {
    Unavailable,
    ReadOnly,
}

impl From<i32> for ShadowMode {
    /// from converts a i32 value to ShadowMode entry.
    fn from(value: i32) -> Self {
        match value {
            0 => Self::Unavailable,
            1 => Self::ReadOnly,
            _ => {
                debug!("Provided shadow mode {value} is not available, using 0 instead");
                Self::Unavailable
            }
        }
    }
}

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

/// Group struct represents a group entry in the cache database.
#[derive(Debug, Serialize)]
pub struct Group {
    pub name: String,
    pub passwd: String,
    pub gid: u32,
    pub members: Vec<String>,
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
    shadow_mode: ShadowMode,
}

/// CacheDBBuilder struct is the struct for the builder pattern and change the parameters of the cache.
pub struct CacheDBBuilder {
    db_path: String,
    root_uid: u32,
    root_gid: u32,
    shadow_gid: i32,
}

impl Default for CacheDBBuilder {
    /// default sets the default values for the CacheDBBuilder.
    fn default() -> Self {
        Self {
            db_path: DB_PATH.to_string(),
            root_uid: 0,
            root_gid: 0,
            shadow_gid: -1,
        }
    }
}

/// DbFileInfo struct represents the expected ownership and permissions for the database file.
struct DbFileInfo {
    path: PathBuf,
    expected_uid: u32,
    expected_gid: u32,
    expected_perms: Permissions,
}

#[allow(dead_code)]
impl CacheDBBuilder {
    /// with_db_path overrides the path to the cache database.
    pub fn with_db_path(&mut self, db_path: &str) -> &mut Self {
        debug!("using custom db path: {}", db_path);
        self.db_path = db_path.to_string();
        self
    }

    /// with_root_uid overrides the default root uid for the cache database.
    pub fn with_root_uid(&mut self, uid: u32) -> &mut Self {
        debug!("using custom root uid '{uid}'");
        self.root_uid = uid;
        self
    }

    /// with_root_gid overrides the default root gid for the cache database.
    pub fn with_root_gid(&mut self, gid: u32) -> &mut Self {
        debug!("using custom root gid '{gid}'");
        self.root_gid = gid;
        self
    }

    /// with_shadow_gid overrides the default shadow gid for the cache database.
    pub fn with_shadow_gid(&mut self, shadow_gid: i32) -> &mut Self {
        debug!("using custom shadow gid '{shadow_gid}'");
        self.shadow_gid = shadow_gid;
        self
    }

    /// build initializes and opens a connection to the cache database.
    pub fn build(&mut self) -> Result<CacheDB, CacheError> {
        debug!("opening database connection from {}", self.db_path);

        if self.shadow_gid < 0 {
            let gid = match users::get_group_by_name("shadow") {
                Some(group) => group.gid(),
                None => {
                    return Err(CacheError::DatabaseError(
                        "failed to find group id for group 'shadow'".to_string(),
                    ))
                }
            };
            self.shadow_gid = gid as i32;
        }

        let db_path = Path::new(&self.db_path);
        let db_files: Vec<DbFileInfo> = vec![
            DbFileInfo {
                // PASSWD
                path: db_path.join(PASSWD_DB),
                expected_uid: self.root_uid,
                expected_gid: self.root_gid,
                expected_perms: Permissions::from_mode(0o644),
            },
            DbFileInfo {
                // SHADOW
                path: db_path.join(SHADOW_DB),
                expected_uid: self.root_uid,
                expected_gid: self.shadow_gid as u32,
                expected_perms: Permissions::from_mode(0o640),
            },
        ];
        Self::check_file_permissions(&db_files)?;

        let passwd_db = &db_path.join(PASSWD_DB);
        let passwd_db = passwd_db.to_str().unwrap();
        let conn = match Connection::open_with_flags(passwd_db, OpenFlags::SQLITE_OPEN_READ_ONLY) {
            Ok(conn) => conn,
            Err(err) => return Err(CacheError::DatabaseError(err.to_string())),
        };

        let shadow_db = &db_path.join(SHADOW_DB);
        let mut shadow_mode = ShadowMode::Unavailable;
        if fs::metadata(shadow_db).is_ok() {
            shadow_mode = ShadowMode::ReadOnly;
        }

        // Attaches shadow to the connection if the shadow db is at least ReadOnly for the current user.
        if shadow_mode >= ShadowMode::ReadOnly {
            let shadow_db = shadow_db.to_str().unwrap();

            let stmt_str = format!("attach database '{}' as shadow;", shadow_db);
            if let Err(err) = conn.execute_batch(&stmt_str) {
                return Err(CacheError::DatabaseError(err.to_string()));
            };
        }

        Ok(CacheDB { conn, shadow_mode })
    }

    /// check_file_permissions checks the database files and compares the current ownership and
    /// permissions with the expected ones.
    fn check_file_permissions(files: &Vec<DbFileInfo>) -> Result<(), CacheError> {
        debug!("Checking db file permissions");
        for file in files {
            let stat = match fs::metadata(&file.path) {
                Ok(stat) => stat,
                Err(err) => return Err(CacheError::DatabaseError(err.to_string())),
            };

            // Checks permissions
            if stat.permissions().mode() & file.expected_perms.mode() == 1 {
                return Err(CacheError::DatabaseError(format!(
                    "invalid permissions for {}",
                    file.path.to_str().unwrap()
                )));
            }

            // Checks ownership
            if stat.uid() != file.expected_uid || stat.gid() != file.expected_gid {
                return Err(CacheError::DatabaseError(format!(
                    "invalid ownership for {}",
                    file.path.to_str().unwrap()
                )));
            }
        }

        Ok(())
    }
}

impl CacheDB {
    /// new creates a new CacheDBBuilder object.
    #[allow(clippy::new_ret_no_self)] // builder pattern
    pub fn new() -> CacheDBBuilder {
        CacheDBBuilder {
            ..Default::default()
        }
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

    /// get_all_passwds queries the database for all passwd rows.
    pub fn get_all_passwds(&self) -> Result<Vec<Passwd>, CacheError> {
        let mut stmt = self.prepare_statement(
            "SELECT login, password, uid, gid, gecos, home, shell FROM passwd", // Last empty field is the shadow password
        )?;

        let rows = match stmt.query([]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        Ok(Self::rows_to_passwd_entries(rows))
    }

    /* Group */
    /// get_group_by_gid queries the database for a group entry with matching gid.
    pub fn get_group_by_gid(self: &CacheDB, gid: u32) -> Result<Group, CacheError> {
        // Nested query to avoid the case where the user is not found,
        // then all the values are NULL due to the call to GROUP_CONCAT
        let mut stmt = self.prepare_statement(
            "
            SELECT * FROM (
                SELECT g.name, g.password, g.gid, group_concat(p.login, ',') as members
                FROM groups g, uid_gid u, passwd p
                WHERE g.gid = ?
                AND u.gid = g.gid
                AND p.uid = u.uid
            ) WHERE name IS NOT NULL
            ",
        )?;

        let rows = match stmt.query([gid]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        let mut entries = Self::rows_to_group_entries(rows);
        Self::expect_one_row(&mut entries)
    }

    /// get_group_by_name queries the database for a group with matching name.
    pub fn get_group_by_name(self: &CacheDB, name: &str) -> Result<Group, CacheError> {
        // Nested query to avoid the case where the user is not found,
        // then all the values are NULL due to the call to GROUP_CONCAT
        let mut stmt = self.prepare_statement(
            "
            SELECT * FROM (
                SELECT g.name, g.password, g.gid, group_concat(p.login, ',') as members
                FROM groups g, uid_gid u, passwd p
                WHERE g.name = ?
                AND u.gid = g.gid
                AND p.uid = u.uid
            ) WHERE name IS NOT NULL
            ",
        )?;

        let rows = match stmt.query([name]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        let mut entries = Self::rows_to_group_entries(rows);
        Self::expect_one_row(&mut entries)
    }

    /// get_all_groups queries the database for all groups.
    pub fn get_all_groups(self: &CacheDB) -> Result<Vec<Group>, CacheError> {
        let mut stmt = self.prepare_statement(
            "
            SELECT * FROM (
                SELECT g.name, g.password, g.gid, group_concat(p.login, ',') as members
                FROM groups g, uid_gid u, passwd p
                WHERE u.gid = g.gid
                AND p.uid = u.uid
            ) WHERE name IS NOT NULL
            ",
        )?;

        let rows = match stmt.query([]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        Ok(Self::rows_to_group_entries(rows))
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

    /// rows_to_group_entries converts SQL rows to a `Vec<Group>`.
    fn rows_to_group_entries(mut rows: Rows) -> Vec<Group> {
        let mut entries = Vec::new();
        while let Ok(Some(row)) = rows.next() {
            let mut members: Vec<String> = Vec::new();

            let tmp: String = row.get(3).expect("invalid members");
            for member in tmp.split(',') {
                members.push(member.to_string());
            }

            entries.push(Group {
                name: row.get(0).expect("invalid name"),
                passwd: row.get(1).expect("invalid passwd"),
                gid: row.get(2).expect("invalid gid"),
                members,
            })
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
