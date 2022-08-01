package shadow

import (
	"github.com/ubuntu/aad-auth/internal/cache"
	"gopkg.in/yaml.v3"
)

// SetCacheOption set opts everytime we open a cache.
// This is not compatible with parallel testing as it needs to change a global state.
func SetCacheOption(opts ...cache.Option) {
	testopts = opts
}

type publicShadow struct {
	Name   string
	Passwd string
	Lstchg int
	Min    int
	Max    int
	Warn   int
	Inact  int
	Expire int
}

// MarshalYAML use a public object to Marhsal to a yaml format.
func (s Shadow) MarshalYAML() (interface{}, error) {
	return publicShadow{
		Name:   s.name,
		Passwd: s.passwd,
		Lstchg: s.lstchg,
		Min:    s.min,
		Max:    s.max,
		Warn:   s.warn,
		Inact:  s.inact,
		Expire: s.expire,
	}, nil
}

// UnmarshalYAML use a public object to Unmarhsal to.
func (s *Shadow) UnmarshalYAML(value *yaml.Node) error {
	o := publicShadow{}
	err := value.Decode(&o)
	if err != nil {
		return err
	}

	*s = Shadow{
		name:   o.Name,
		passwd: o.Passwd,
		lstchg: o.Lstchg,
		min:    o.Min,
		max:    o.Max,
		warn:   o.Warn,
		inact:  o.Inact,
		expire: o.Expire,
	}
	return nil
}

// NewTestShadow return a new Shadow entry for tests.
func NewTestShadow() Shadow {
	return Shadow{
		name:   "testusername@domain.com",
		passwd: "*",
		lstchg: -1,
		min:    -1,
		max:    -1,
		warn:   -1,
		expire: -1,
		inact:  -1,
	}
}
