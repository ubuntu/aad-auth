// Package consts includes important constants used in the project.
package consts

import log "github.com/sirupsen/logrus"

var (
	// Version is the version of the executable.
	Version = "dev"
)

const (
	// DefaultConfigPath is the default path to the config file.
	DefaultConfigPath = "/etc/aad.conf"

	// TEXTDOMAIN is the gettext domain for l10n.
	TEXTDOMAIN = "aad-auth"

	// DefaultLogLevel is the default logging level when no option is passed.
	DefaultLogLevel = log.WarnLevel

	// DefaultEditor is the default editor to use when no option is passed.
	DefaultEditor = "sensible-editor"
)
