package cache

var (
	// PasswdSQLForTests is the sql used to create the passwd database.
	PasswdSQLForTests = sqlCreatePasswdTables
	// ShadowSQLForTests is the sql used to create the shadow database.
	ShadowSQLForTests = sqlCreateShadowTables
)
