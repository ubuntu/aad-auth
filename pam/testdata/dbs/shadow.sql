CREATE TABLE IF NOT EXISTS shadow (
	uid             INTEGER NOT NULL UNIQUE,
	password        TEXT    NOT NULL,
	last_pwd_change	INTEGER NOT NULL DEFAULT -1,  -- -1 = Empty value: It disables the functionality, 0 change password on next login
	min_pwd_age     INTEGER NOT NULL DEFAULT -1,  -- 0 no minimum age
	max_pwd_age     INTEGER NOT NULL DEFAULT -1,  -- NULL disabled
	pwd_warn_period	INTEGER NOT NULL DEFAULT -1,
	pwd_inactivity	INTEGER NOT NULL DEFAULT -1,
	expiration_date	INTEGER NOT NULL DEFAULT -1,
	PRIMARY KEY("uid")
);