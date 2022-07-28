package nss

const (
	// NssLogEnv is the env variable name to force debug
	NssLogEnv = nssLogEnv
)

// WithDebug forces debug mode, whatever environment variable is set.
func WithDebug() func(o *options) {
	return func(o *options) {
		o.debug = true
	}
}

// WithLogWriter override the syslog writer we assign.
func WithLogWriter(w logWriter) func(o *options) {
	return func(o *options) {
		o.writer = w
	}
}
