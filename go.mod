module github.com/ubuntu/aad-auth

go 1.22.0

toolchain go1.22.1

require (
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2
	github.com/go-ini/ini v1.67.0
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/stretchr/testify v1.9.0
	github.com/ubuntu/decorate v0.0.0-20230125165522-2d5b0a9bb117
	golang.org/x/crypto v0.21.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/msteinert/pam v1.0.0
	github.com/muesli/mango-cobra v1.2.0
	github.com/muesli/roff v0.1.0
	github.com/sirupsen/logrus v1.9.3
	github.com/snapcore/go-gettext v0.0.0-20201130093759-38740d1bd3d2
	github.com/spf13/cobra v1.8.0
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e
	golang.org/x/sys v0.18.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.0.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/muesli/mango v0.1.0 // indirect
	github.com/muesli/mango-pflag v0.1.0 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace github.com/msteinert/pam => github.com/didrocks/pam v0.0.0-20220802135005-32a8a9a45248
