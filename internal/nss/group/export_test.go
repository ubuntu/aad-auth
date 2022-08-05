package group

import (
	"fmt"

	"github.com/ubuntu/aad-auth/internal/cache"
	"gopkg.in/yaml.v3"
)

// SetCacheOption set opts everytime we open a cache.
// This is not compatible with parallel testing as it needs to change a global state.
func SetCacheOption(opts ...cache.Option) {
	testopts = opts
}

type publicGroup struct {
	Name    string
	Passwd  string
	GID     uint
	Members []string
}

// MarshalYAML use a public object to Marhsal to a yaml format.
func (g Group) MarshalYAML() (interface{}, error) {
	return publicGroup{
		Name:    g.name,
		Passwd:  g.passwd,
		GID:     g.gid,
		Members: g.members,
	}, nil
}

// UnmarshalYAML use a public object to Unmarhsal to.
func (g *Group) UnmarshalYAML(value *yaml.Node) error {
	o := publicGroup{}
	err := value.Decode(&o)
	if err != nil {
		return err
	}

	*g = Group{
		name:    o.Name,
		passwd:  o.Passwd,
		gid:     o.GID,
		members: o.Members,
	}
	return nil
}

// NewTestGroup return a new Group entry for tests.
func NewTestGroup(nMembers int) Group {
	members := make([]string, 0, nMembers)

	members = append(members, fmt.Sprint("testusername@domain.com"))
	for i := 1; i < nMembers; i++ {
		members = append(members, fmt.Sprintf("testusername-%d@domain.com", i))
	}

	return Group{
		name:    "testusername@domain.com",
		passwd:  "x",
		gid:     2345,
		members: members,
	}
}
