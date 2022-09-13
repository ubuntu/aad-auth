package cache

var (
	// PasswdSQLForTests is the sql used to create the passwd database.
	PasswdSQLForTests = sqlCreatePasswdTables
	// ShadowSQLForTests is the sql used to create the shadow database.
	ShadowSQLForTests = sqlCreateShadowTables
	// DefaultCredentialsExpiration is the default number of days the user is allowed to login without online revalidation.
	DefaultCredentialsExpiration = defaultCredentialsExpiration
)
