module github.com/ubuntu/aad-auth

go 1.20

require (
	github.com/AzureAD/microsoft-authentication-library-for-go v0.8.1
	github.com/go-ini/ini v1.67.0
	github.com/mattn/go-sqlite3 v1.14.16
	github.com/stretchr/testify v1.8.2
	golang.org/x/crypto v0.6.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/msteinert/pam v1.0.0
	github.com/muesli/mango-cobra v1.2.0
	github.com/muesli/roff v0.1.0
	github.com/sirupsen/logrus v1.9.0
	github.com/snapcore/go-gettext v0.0.0-20201130093759-38740d1bd3d2
	github.com/spf13/cobra v1.6.1
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e
	golang.org/x/sys v0.6.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/muesli/mango v0.1.0 // indirect
	github.com/muesli/mango-pflag v0.1.0 // indirect
	github.com/pkg/browser v0.0.0-20210115035449-ce105d075bb4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace github.com/msteinert/pam => github.com/didrocks/pam v0.0.0-20220802135005-32a8a9a45248
