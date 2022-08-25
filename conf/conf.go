// Package conf includes the embedded configuration template for the application.
package conf

import _ "embed"

// AADConfTemplate holds the template for the AAD configuration file.
//
//go:embed aad.conf.template
var AADConfTemplate string
