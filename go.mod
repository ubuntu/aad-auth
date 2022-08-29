module github.com/ubuntu/aad-auth

go 1.18

require (
	github.com/AzureAD/microsoft-authentication-library-for-go v0.5.3
	github.com/go-ini/ini v1.67.0
	github.com/mattn/go-sqlite3 v1.14.15
	github.com/stretchr/testify v1.8.0
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa
	gopkg.in/yaml.v3 v3.0.1
)

require github.com/msteinert/pam v1.0.0

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang-jwt/jwt v3.2.1+incompatible // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/pkg/browser v0.0.0-20210115035449-ce105d075bb4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f // indirect
)

replace github.com/msteinert/pam => github.com/didrocks/pam v0.0.0-20220802135005-32a8a9a45248
