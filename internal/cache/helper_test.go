package cache_test

type userInfos struct {
	name     string
	uid      int64
	password string
}

var (
	usersForTests = map[string]userInfos{
		"myuser@domain.com":    {"myuser@domain.com", 1929326240, "my password"},
		"otheruser@domain.com": {"otheruser@domain.com", 165119648, "other password"},
		"user@otherdomain.com": {"user@otherdomain.com", 165119649, "other user domain password"},
	}
	usersForTestsByUID = make(map[uint]userInfos)
)

func init() {
	// populate usersForTestByUid
	for _, info := range usersForTests {
		usersForTestsByUID[uint(info.uid)] = info
	}
}
