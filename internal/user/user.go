package user

import "strings"

// NormalizeName returned a normalized, lowercase version of the username as
// AnYCaSe@DomAIN is accepted by aad.
func NormalizeName(name string) string {
	return strings.ToLower(name)
}
