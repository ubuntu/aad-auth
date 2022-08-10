package group

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

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
	if err := value.Decode(&o); err != nil {
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

	members = append(members, "testusername@domain.com")
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
