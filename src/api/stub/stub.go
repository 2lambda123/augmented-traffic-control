package stub

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"atc_thrift"
)

var (
	FakeShaping = &atc_thrift.Shaping{
		Up: &atc_thrift.LinkShaping{
			Rate:       1024,
			Delay:      nil,
			Loss:       nil,
			Reorder:    nil,
			Corruption: nil,
		},
		Down: &atc_thrift.LinkShaping{
			Rate:       1024,
			Delay:      nil,
			Loss:       nil,
			Reorder:    nil,
			Corruption: nil,
		},
	}

	FakeGroup = &atc_thrift.ShapingGroup{
		Id:      15,
		Members: []string{"1.2.3.4", "5.6.7.8"},
		Shaping: FakeShaping,
	}
)

type fake_atcd struct {
	nextId int64
	groups map[int64]*atc_thrift.ShapingGroup
}

func NewFakeAtcd() *fake_atcd {
	atcd := &fake_atcd{
		nextId: 16,
		groups: make(map[int64]*atc_thrift.ShapingGroup),
	}
	atcd.groups[FakeGroup.Id] = FakeGroup
	return atcd
}

func (atcd *fake_atcd) GetAtcdInfo() (*atc_thrift.AtcdInfo, error) {
	info := &atc_thrift.AtcdInfo{
		Platform: atc_thrift.PlatformType_LINUX,
		Version:  "2.0-gostub",
	}
	return info, nil
}

func (atcd *fake_atcd) ListGroups() ([]*atc_thrift.ShapingGroup, error) {
	groups := make([]*atc_thrift.ShapingGroup, len(atcd.groups))
	for i, g := range atcd.groups {
		groups[i] = g
	}
	return groups, nil
}

func (atcd *fake_atcd) CreateGroup(member string) (*atc_thrift.ShapingGroup, error) {
	defer func() { atcd.nextId++ }()
	id := atcd.nextId
	atcd.groups[id] = &atc_thrift.ShapingGroup{
		Id:      id,
		Members: []string{member},
		Shaping: nil,
	}
	return atcd.groups[id], nil
}

func (atcd *fake_atcd) GetGroup(id int64) (*atc_thrift.ShapingGroup, error) {
	grp, ok := atcd.groups[id]
	if !ok {
		return nil, fmt.Errorf("Group not found: %d", id)
	}
	return grp, nil
}

func (atcd *fake_atcd) GetGroupWith(member string) (*atc_thrift.ShapingGroup, error) {
	for _, grp := range atcd.groups {
		for _, addr := range grp.Members {
			if addr == member {
				return grp, nil
			}
		}
	}
	return nil, fmt.Errorf("Group not found with member %q", member)
}

func (atcd *fake_atcd) token(id int64) string {
	// NOT CRYPTOGRAPHICALLY SECURE
	// Do not use this in production!
	t := time.Now().Unix() / 30
	s := fmt.Sprintf("%d%d", id, t)
	sum := sha1.Sum([]byte(s))
	dst := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(dst, sum[:])
	return string(dst)
}

func (atcd *fake_atcd) check_token(id int64, token string) bool {
	return atcd.token(id) == token
}

func (atcd *fake_atcd) GetGroupToken(id int64) (string, error) {
	_, ok := atcd.groups[id]
	if !ok {
		return "", fmt.Errorf("Group not found: %d", id)
	}
	return atcd.token(id), nil
}

func (atcd *fake_atcd) JoinGroup(id int64, to_add string, token string) error {
	grp, ok := atcd.groups[id]
	if !ok {
		return fmt.Errorf("Group not found: %d", id)
	}
	if !atcd.check_token(id, token) {
		return fmt.Errorf("Invalid token")
	}
	grp.Members = append(grp.Members, to_add)
	return nil
}

func (atcd *fake_atcd) LeaveGroup(id int64, to_remove string, token string) error {
	grp, ok := atcd.groups[id]
	if !ok {
		return fmt.Errorf("Group not found: %d", id)
	}
	if !atcd.check_token(id, token) {
		return fmt.Errorf("Invalid token")
	}
	removed := false
	members := make([]string, 0, len(grp.Members))
	for _, member := range grp.Members {
		if member != to_remove {
			members = append(members, member)
		} else {
			removed = true
		}
	}
	if !removed {
		return fmt.Errorf("Member not found in group: %q", to_remove)
	}
	grp.Members = members
	return nil
}

func (atcd *fake_atcd) ShapeGroup(id int64, settings *atc_thrift.Shaping, token string) (*atc_thrift.Shaping, error) {
	grp, ok := atcd.groups[id]
	if !ok {
		return nil, fmt.Errorf("Group not found: %d", id)
	}
	if !atcd.check_token(id, token) {
		return nil, fmt.Errorf("Invalid token")
	}
	grp.Shaping = settings
	return grp.Shaping, nil
}

func (atcd *fake_atcd) UnshapeGroup(id int64, token string) error {
	grp, ok := atcd.groups[id]
	if !ok {
		return fmt.Errorf("Group not found: %d", id)
	}
	if !atcd.check_token(id, token) {
		return fmt.Errorf("Invalid token")
	}
	grp.Shaping = nil
	return nil
}

// This is a convenience method for allowing us to use a *fake_atcd as an
// Atcd client connection
func (atcd *fake_atcd) Close() {
	// Do nothing
}
