use std::{
    fmt::Debug,
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

const PASSWD_DB: &str = "passwd.db"; // Ownership: root:root
pub const PASSWD_PERMS: u32 = 0o644;

const SHADOW_DB: &str = "shadow.db"; // Ownership: root:shadow
pub const SHADOW_PERMS: u32 = 0o640;

/// ShadowMode enum represents the status of the shadow database.
#[derive(PartialEq, PartialOrd, Debug, Clone, Copy)]
pub enum ShadowMode {
    Unavailable,
    ReadOnly,
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

/// Shadow struct represents a shadow entry in the cache database.
#[derive(Serialize)]
pub struct Shadow {
    pub name: String,
    pub passwd: String,
    pub last_pwd_change: i64,
    pub min_pwd_age: i64,
    pub max_pwd_age: i64,
    pub pwd_warn_period: i64,
    pub pwd_inactivity: i64,
    pub expiration_date: i64,
}

impl Debug for Shadow {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Shadow")
            .field("name", &self.name)
            .field("passwd", &"REDACTED")
            .field("last_pwd_change", &self.last_pwd_change)
            .field("min_pwd_age", &self.min_pwd_age)
            .field("max_pwd_age", &self.max_pwd_age)
            .field("pwd_warn_period", &self.pwd_warn_period)
            .field("pwd_inactivity", &self.pwd_inactivity)
            .field("expiration_date", &self.expiration_date)
            .finish()
    }
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
    /// db_path is the path in which the databases will be created.
    db_path: String,

    /// root_uid is the uid of the database owner.
    root_uid: u32,
    /// root_gid is the gid of the owner group.
    root_gid: u32,
    /// shadow_gid is the gid to be used by the shadow group.
    shadow_gid: Option<u32>,
}

/// DbFileInfo struct represents the expected ownership and permissions for the database file.
struct DbFileInfo {
    path: PathBuf,
    expected_uid: u32,
    expected_gid: u32,
    expected_perms: Permissions,
}

impl CacheDBBuilder {
    /// with_db_path overrides the path to the cache database.
    pub fn with_db_path(&mut self, db_path: &str) -> &mut Self {
        debug!("using custom db path: {}", db_path);
        self.db_path = db_path.to_string();
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(test)]
    /// with_root_uid overrides the default root uid for the cache database.
    pub fn with_root_uid(&mut self, uid: u32) -> &mut Self {
        debug!("using custom root uid '{uid}'");
        self.root_uid = uid;
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(test)]
    /// with_root_gid overrides the default root gid for the cache database.
    pub fn with_root_gid(&mut self, gid: u32) -> &mut Self {
        debug!("using custom root gid '{gid}'");
        self.root_gid = gid;
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(test)]
    /// with_shadow_gid overrides the default shadow gid for the cache database.
    pub fn with_shadow_gid(&mut self, shadow_gid: u32) -> &mut Self {
        debug!("using custom shadow gid '{shadow_gid}'");
        self.shadow_gid = Some(shadow_gid);
        self
    }

    /// build initializes and opens a connection to the cache database.
    pub fn build(&mut self) -> Result<CacheDB, CacheError> {
        debug!("opening database connection from {}", self.db_path);

        let shadow_gid = if let Some(gid) = self.shadow_gid {
            gid
        } else {
            // If shadow_gid is not set, we auto detect it from the shadow group.
            match users::get_group_by_name("shadow") {
                Some(group) => group.gid(),
                None => {
                    return Err(CacheError::DatabaseError(
                        "failed to find group id for group 'shadow'".to_string(),
                    ))
                }
            }
        };

        let db_path = Path::new(&self.db_path);
        let db_files: Vec<DbFileInfo> = vec![
            DbFileInfo {
                // PASSWD
                path: db_path.join(PASSWD_DB),
                expected_uid: self.root_uid,
                expected_gid: self.root_gid,
                expected_perms: Permissions::from_mode(PASSWD_PERMS),
            },
            DbFileInfo {
                // SHADOW
                path: db_path.join(SHADOW_DB),
                expected_uid: self.root_uid,
                expected_gid: shadow_gid,
                expected_perms: Permissions::from_mode(SHADOW_PERMS),
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

            let stmt_str = format!("attach database '{shadow_db}' as shadow;");
            if let Err(err) = conn.execute_batch(&stmt_str) {
                return Err(CacheError::DatabaseError(err.to_string()));
            };
        }

        Ok(CacheDB { conn, shadow_mode })
    }

    /// check_file_permissions checks the database files and compares the current ownership and
    /// permissions with the expected ones.
    fn check_file_permissions(files: &Vec<DbFileInfo>) -> Result<(), CacheError> {
        for file in files {
            debug!("Checking file {:?} permissions", file.path);
            let stat = match fs::metadata(&file.path) {
                Ok(st) => st,
                Err(err) => return Err(CacheError::DatabaseError(err.to_string())),
            };

            // Checks permissions
            if stat.permissions().mode() & file.expected_perms.mode() != file.expected_perms.mode()
            {
                return Err(CacheError::DatabaseError(format!(
                    "invalid permissions for {}, expected {:o} but got {:o}",
                    file.path.to_str().unwrap(),
                    file.expected_perms.mode(),
                    stat.permissions().mode()
                )));
            }

            // Checks ownership
            if stat.uid() != file.expected_uid || stat.gid() != file.expected_gid {
                return Err(CacheError::DatabaseError(format!(
                    "invalid ownership for {}, expected {}:{} but got {}:{}",
                    file.path.to_str().unwrap(),
                    file.expected_uid,
                    file.expected_gid,
                    stat.uid(),
                    stat.gid()
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
            db_path: DB_PATH.to_string(),
            root_uid: 0,
            root_gid: 0,
            shadow_gid: None,
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

    /* Shadow */
    /// get_shadow_by_name queries the database for a shadow row with matching name.
    pub fn get_shadow_by_name(&self, name: &str) -> Result<Shadow, CacheError> {
        if self.shadow_mode < ShadowMode::ReadOnly {
            return Err(CacheError::DatabaseError(
                "Shadow database is not accessible".to_string(),
            ));
        }

        let mut stmt = self.prepare_statement(
            "
            SELECT p.login, s.password, s.last_pwd_change, s.min_pwd_age, s.max_pwd_age, s.pwd_warn_period, s.pwd_inactivity, s.expiration_date
            FROM passwd p, shadow.shadow s
            WHERE p.uid = s.uid
            AND p.login = ?
            "
        )?;

        let rows = match stmt.query([name]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        let mut entries = Self::rows_to_shadow_entries(rows);

        Self::expect_one_row(&mut entries)
    }

    /// get_all_shadows queries the database for all shadow rows.
    pub fn get_all_shadows(&self) -> Result<Vec<Shadow>, CacheError> {
        if self.shadow_mode < ShadowMode::ReadOnly {
            return Err(CacheError::DatabaseError(
                "Shadow database is not accessible".to_string(),
            ));
        }

        let mut stmt = self.prepare_statement(
            "
            SELECT p.login, s.password, s.last_pwd_change, s.min_pwd_age, s.max_pwd_age, s.pwd_warn_period, s.pwd_inactivity, s.expiration_date
            FROM passwd p, shadow.shadow s
            WHERE p.uid = s.uid
            "
        )?;

        let rows = match stmt.query([]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        Ok(Self::rows_to_shadow_entries(rows))
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

    /// rows_to_shadow_entries converts SQL rows to `Vec<Shadow>`.
    fn rows_to_shadow_entries(mut rows: Rows) -> Vec<Shadow> {
        let mut entries = Vec::new();
        while let Ok(Some(row)) = rows.next() {
            entries.push(Shadow {
                name: row.get(0).expect("invalid login"),
                passwd: row.get(1).expect("invalid passwd"),
                last_pwd_change: row.get(2).expect("invalid last_pwd_change"),
                min_pwd_age: row.get(3).expect("invalid min_pwd_age"),
                max_pwd_age: row.get(4).expect("invalid max_pwd_age"),
                pwd_warn_period: row.get(5).expect("invalid pwd_warn_period"),
                pwd_inactivity: row.get(6).expect("invalid pwd_inactivity"),
                expiration_date: row.get(7).expect("invalid expiration_date"),
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
