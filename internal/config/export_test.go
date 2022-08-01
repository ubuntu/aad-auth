package config

// TODO: comment
func WithAddUserConfPath(path string) Option {
	return func(o *options) {
		if path != "" {
			o.addUserConfPath = path
		}
	}
}
