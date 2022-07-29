package shadow

import (
	"github.com/ubuntu/aad-auth/internal/cache"
)

// SetCacheOption set opts everytime we open a cache.
// This is not compatible with parallel testing as it needs to change a global state.
func SetCacheOption(opts ...cache.Option) {
	testopts = opts
}
