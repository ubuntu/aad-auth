use faccess::PathExt;
use rusqlite::{Connection, OpenFlags, Rows, Statement};
use serde::Serialize;
use std::{
    fmt::Debug,
    fs::{self, Permissions},
    os::unix::fs::MetadataExt,
    os::unix::fs::PermissionsExt,
    path::{Path, PathBuf},
};
use time::{Duration, OffsetDateTime};

use crate::debug;

#[cfg(test)]
#[allow(clippy::too_many_arguments)]
mod mod_tests;

const DB_PATH: &str = "/var/lib/aad/cache";
pub const OFFLINE_CREDENTIALS_EXPIRATION: i32 = 90;
pub const EXPIRATION_PURGE_MULTIPLIER: i32 = 2;

const PASSWD_DB: &str = "passwd.db"; // Ownership: root:root
pub const PASSWD_PERMS: u32 = 0o644;

const SHADOW_DB: &str = "shadow.db"; // Ownership: root:shadow
pub const SHADOW_PERMS: u32 = 0o640;

const DB_CONN_PREFIX: &str = "file:"; // Specify "file" as access mode, so that we can use connection options.
const DB_CONN_OPT_RW: &str = "?journal_mode=wal"; // Use Write Ahead Log journaling mode, so that we can operate paralell with the PAM module.
const DB_CONN_OPT_RO: &str = "?immutable=1"; // When using immutable=1, we can still read the passwd_db, even if there is an db lock.

/// ShadowMode enum represents the status of the shadow database.
#[derive(PartialEq, PartialOrd, Debug, Clone, Copy)]
pub enum ShadowMode {
    AutoDetect = -1,
    Unavailable,
    ReadOnly,
    ReadWrite,
}
impl From<i32> for ShadowMode {
    fn from(value: i32) -> Self {
        match value {
            -1 => ShadowMode::AutoDetect,
            0 => ShadowMode::Unavailable,
            1 => ShadowMode::ReadOnly,
            2 => ShadowMode::ReadWrite,
            other => {
                debug!("Unrecognized mode {other}, using 0 instead");
                ShadowMode::Unavailable
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

/// Shadow struct represents a shadow entry in the cache database.
#[derive(Serialize)]
pub struct Shadow {
    pub name: String,
    pub passwd: String,
    pub last_pwd_change: isize,
    pub min_pwd_age: isize,
    pub max_pwd_age: isize,
    pub pwd_warn_period: isize,
    pub pwd_inactivity: isize,
    pub expiration_date: isize,
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
    NoDatabases(String),
    DatabaseError(String),
    QueryError(String),
    NoRecord,
}

/// CacheDB struct represents the cache database.
#[cfg_attr(test, derive(Debug))]
pub struct CacheDB {
    conn: Connection,
    shadow_mode: ShadowMode,

    /// offline_credentials_expiration is the number of days a user will be allowed to login without
    /// online authentication.
    ///
    /// Users that have not authenticated online for more than this ammount of days will be prevented
    /// from offline authentication and purged from the cache if the days without online authentication
    /// exceed twice this ammount.
    offline_credentials_expiration: i32,
}

/// CacheDBBuilder struct is the struct for the builder pattern and change the parameters of the cache.
pub struct CacheDBBuilder {
    /// db_path is the path in which the databases will be created.
    db_path: String,
    /// offline_credentials_expiration is the number of days a user will be allowed to login without
    /// online authentication.
    ///
    /// Users that have not authenticated online for more than this ammount of days will be prevented
    /// from offline authentication and purged from the cache if the days without online authentication
    /// exceed twice this ammount.
    offline_credentials_expiration: i32,
    /// root_uid is the uid of the database owner.
    root_uid: u32,
    /// root_gid is the gid of the owner group.
    root_gid: u32,
    /// shadow_gid is the gid to be used by the shadow group.
    shadow_gid: Option<u32>,
    /// shadow_mode is the manual access level to be used by the shadow database (for tests)
    shadow_mode: ShadowMode,
    /// passwd_perms is the default expected permissions for the passwd db file.
    passwd_perms: Permissions,
    /// shadow_perms is the default expected permissions for the shadow db file.
    shadow_perms: Permissions,
}

/// DbFileInfo struct represents the expected ownership and permissions for the database file.
struct DbFileInfo<'a> {
    path: PathBuf,
    expected_uid: u32,
    expected_gid: u32,
    expected_perms: &'a Permissions,
}

impl CacheDBBuilder {
    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(any(feature = "integration-tests", test))]
    /// with_db_path overrides the path to the cache database.
    pub fn with_db_path(&mut self, db_path: &str) -> &mut Self {
        debug!("using custom db path: {}", db_path);
        self.db_path = db_path.to_string();
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(any(feature = "integration-tests", test))]
    /// with_root_uid overrides the default root uid for the cache database.
    pub fn with_root_uid(&mut self, uid: u32) -> &mut Self {
        debug!("using custom root uid '{uid}'");
        self.root_uid = uid;
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(any(feature = "integration-tests", test))]
    /// with_root_gid overrides the default root gid for the cache database.
    pub fn with_root_gid(&mut self, gid: u32) -> &mut Self {
        debug!("using custom root gid '{gid}'");
        self.root_gid = gid;
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(any(feature = "integration-tests", test))]
    /// with_shadow_gid overrides the default shadow gid for the cache database.
    pub fn with_shadow_gid(&mut self, shadow_gid: u32) -> &mut Self {
        debug!("using custom shadow gid '{shadow_gid}'");
        self.shadow_gid = Some(shadow_gid);
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(any(feature = "integration-tests", test))]
    /// with_shadow_mode overrides the default access level for the shadow database.
    pub fn with_shadow_mode(&mut self, shadow_mode: i32) -> &mut Self {
        debug!("using custom shadow mode '{shadow_mode}'");
        self.shadow_mode = ShadowMode::from(shadow_mode);
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(test)]
    /// with_offline_credentials_expiration overrides the default ammount of time a user can authenticate
    /// without online verification.
    pub fn with_offline_credentials_expiration(&mut self, value: i32) -> &mut Self {
        debug!("using custom credentials expiration '{value}'");
        self.offline_credentials_expiration = value;
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(test)]
    /// with_passwd_perms overrides the default expected permissions for the passwd database.
    pub fn with_passwd_perms(&mut self, value: u32) -> &mut Self {
        debug!("using custom passwd permissions '{value}'");
        self.passwd_perms = Permissions::from_mode(value);
        self
    }

    // This is a function to be used in tests, so we need to annotate it.
    #[cfg(test)]
    /// with_shadow_perms overrides the default expected permissions for the shadow database.
    pub fn with_shadow_perms(&mut self, value: u32) -> &mut Self {
        debug!("using custom shadow permissions '{value}'");
        self.shadow_perms = Permissions::from_mode(value);
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
                expected_perms: &self.passwd_perms,
            },
            DbFileInfo {
                // SHADOW
                path: db_path.join(SHADOW_DB),
                expected_uid: self.root_uid,
                expected_gid: shadow_gid,
                expected_perms: &self.shadow_perms,
            },
        ];

        // Checks if at least one of db files exist. If none is found, then we consider it as first
        // time access.
        let mut found = false;
        for file in &db_files {
            if file.path.exists() {
                found = true;
            }
        }
        if !found {
            return Err(CacheError::NoDatabases(format!(
                "no aad-auth cache found at {db_path:?}"
            )));
        }

        Self::check_file_permissions(&db_files)?;

        let shadow_db = &db_path.join(SHADOW_DB);
        let mut shadow_mode = self.shadow_mode;
        if shadow_mode == ShadowMode::AutoDetect {
            shadow_mode = ShadowMode::Unavailable;
            if shadow_db.readable() {
                shadow_mode = ShadowMode::ReadOnly;
            }
            if shadow_db.writable() {
                shadow_mode = ShadowMode::ReadWrite;
            }
        }

        let open_flags = match shadow_mode {
            ShadowMode::ReadWrite => OpenFlags::SQLITE_OPEN_READ_WRITE,
            _ => OpenFlags::SQLITE_OPEN_READ_ONLY,
        };

        let passwd_db = &db_path.join(PASSWD_DB);

        let mut passwd_db_args = DB_CONN_OPT_RO;
        if passwd_db.writable(){ 
            // db_path must also be writeable for 'journal_mode=wal';
            if db_path.writable(){
                passwd_db_args = DB_CONN_OPT_RW;
            }
        }
        let passwd_db = passwd_db.to_str().unwrap();
        let passwd_db_conn = format!("{}{}{}", DB_CONN_PREFIX, passwd_db, passwd_db_args);

        debug!("Opening database: {passwd_db_conn}");
        let conn = match Connection::open_with_flags(passwd_db_conn, open_flags) {
            Ok(conn) => conn,
            Err(err) => return Err(CacheError::DatabaseError(err.to_string())),
        };

        // Attaches shadow to the connection if the shadow db is at least ReadOnly for the current user.
        if shadow_mode >= ShadowMode::ReadOnly {
            let shadow_db = shadow_db.to_str().unwrap();

            let stmt_str = format!("attach database '{shadow_db}' as shadow;");
            if let Err(err) = conn.execute_batch(&stmt_str) {
                return Err(CacheError::DatabaseError(err.to_string()));
            };
        }

        let mut c = CacheDB {
            conn,
            shadow_mode,
            offline_credentials_expiration: self.offline_credentials_expiration,
        };

        if shadow_mode >= ShadowMode::ReadWrite {
            if let Err(err) = c.cleanup_expired_entries() {
                return Err(CacheError::DatabaseError(err.to_string()));
            }
        }

        Ok(c)
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
            offline_credentials_expiration: OFFLINE_CREDENTIALS_EXPIRATION,
            root_uid: 0,
            root_gid: 0,
            shadow_gid: None,
            shadow_mode: ShadowMode::AutoDetect,
            passwd_perms: Permissions::from_mode(PASSWD_PERMS),
            shadow_perms: Permissions::from_mode(SHADOW_PERMS),
        }
    }

    /// new_for_tests creates a new CacheDBBuilder object to be used in tests.
    #[allow(clippy::new_ret_no_self)] // builder pattern
    pub fn new_for_tests() -> CacheDBBuilder {
        // If test profile is not enabled, this function will behave the same as new().
        // For more information, the rationale on lib.rs/new_cache() function.
        if !cfg!(test) {
            return Self::new();
        }

        let mut builder = CacheDBBuilder {
            db_path: std::env::var("NSS_AAD_CACHEDIR").unwrap(),
            offline_credentials_expiration: OFFLINE_CREDENTIALS_EXPIRATION,
            root_uid: users::get_current_uid(),
            root_gid: users::get_current_gid(),
            shadow_gid: Some(users::get_current_gid()),
            shadow_mode: ShadowMode::AutoDetect,
            passwd_perms: Permissions::from_mode(PASSWD_PERMS),
            shadow_perms: Permissions::from_mode(SHADOW_PERMS),
        };

        if let Ok(v) = std::env::var("NSS_AAD_SHADOW_MODE") {
            let tmp = v.parse::<i32>();
            builder.shadow_mode = tmp.unwrap().into();
        }

        builder
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

        let rows = match stmt.query([&Self::normalize_username(login)]) {
            Ok(rows) => rows,
            Err(err) => return Err(CacheError::QueryError(err.to_string())),
        };

        let mut entries = Self::rows_to_passwd_entries(rows);

        Self::expect_one_row(&mut entries)
    }

    /// get_all_passwds queries the database for all passwd rows.
    pub fn get_all_passwds(&self) -> Result<Vec<Passwd>, CacheError> {
        let mut stmt = self.prepare_statement(
            "SELECT login, password, uid, gid, gecos, home, shell FROM passwd ORDER BY login", // Last empty field is the shadow password
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

        let rows = match stmt.query([&Self::normalize_username(name)]) {
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
                GROUP BY g.name
            ) WHERE name IS NOT NULL
            ORDER BY name
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

        let rows = match stmt.query([&Self::normalize_username(name)]) {
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
            ORDER BY p.login
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

    /// cleanup_expired_entries purges all the expired entries from the database.
    fn cleanup_expired_entries(&mut self) -> Result<(), rusqlite::Error> {
        if self.offline_credentials_expiration == 0 {
            debug!("Offline expiration is 0, cache will not be cleaned");
            return Ok(());
        }

        let tx = self.conn.transaction()?;

        let days = Duration::days(
            (EXPIRATION_PURGE_MULTIPLIER * self.offline_credentials_expiration).into(),
        );
        let purge_time = (OffsetDateTime::now_utc() - days).unix_timestamp();

        // Shadow cleanup
        tx.execute(
            "DELETE FROM shadow.shadow WHERE uid IN (
                SELECT uid FROM passwd WHERE last_online_auth < ?
            )",
            [purge_time],
        )?;

        // Passwd cleanup
        tx.execute(
            "DELETE FROM passwd WHERE last_online_auth < ?",
            [purge_time],
        )?;

        // Group cleanup
        tx.execute(
            "DELETE FROM groups WHERE gid NOT IN (SELECT DISTINCT gid FROM uid_gid)",
            [],
        )?;

        tx.commit()?;
        Ok(())
    }

    /// normalize_username lowercases the username that is going to be used in a cache query.
    fn normalize_username(username: &str) -> String {
        username.to_lowercase()
    }
}
