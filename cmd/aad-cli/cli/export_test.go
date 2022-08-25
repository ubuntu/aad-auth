package cli

import "github.com/ubuntu/aad-auth/internal/cache"

// WithDpkgQueryCmd specifies a custom dpkg-query command to use for the user command.
// This is only used in tests.
func WithDpkgQueryCmd(p string) func(o *options) {
	return func(o *options) {
		o.dpkgQueryCmd = p
	}
}

// WithCache specifies a personalized cache object to use for the app.
// Useful in tests for overriding the default cache.
func WithCache(c *cache.Cache) func(o *options) {
	return func(o *options) {
		o.cache = c
	}
}

// WithEditor specifies a custom editor to use when editing the config file.
// Will probably only be used in tests.
func WithEditor(p string) func(o *options) {
	return func(o *options) {
		o.editor = p
	}
}

// WithConfigFile specifies a custom config file to use for the config command.
func WithConfigFile(p string) func(o *options) {
	return func(o *options) {
		o.configFile = p
	}
}

// Editor returns the editor used by the program.
func (a App) Editor() string {
	return a.options.editor
}
