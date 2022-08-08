package config

// WithAddUserConfPath overrides /etc/adduser.conf path.
func WithAddUserConfPath(path string) Option {
	return func(o *options) {
		o.addUserConfPath = path
	}
}
