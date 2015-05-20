package types

import (
	"fmt"
	. "github.com/tendermint/tendermint/common"
)

//------------------------------------------------------------------------------------------------

var (
	GlobalPermissionsAddress    = Zero256[:20]
	GlobalPermissionsAddress256 = Zero256
	DougAddress                 = append([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, []byte("THISISDOUG")...)
	DougAddress256              = LeftPadWord256(DougAddress)
)

// A particular permission
type PermFlag uint16

// Base permission references are like unix (the index is already bit shifted)
const (
	Send   PermFlag = 1 << iota // 1
	Call                        // 2
	Create                      // 4
	Bond                        // 8
	Root                        // 16

	DefaultBBPB = Send | Call | Create | Bond

	NumBasePermissions uint     = 5
	TopBasePermission  PermFlag = 1 << (NumBasePermissions - 1)
	AllSet             PermFlag = 1<<NumBasePermissions - 1
)

//---------------------------------------------------------------------------------------------

// Base chain permissions struct
type BasePermissions struct {
	// bit array with "has"/"doesn't have" for each permission
	Perms PermFlag

	// bit array with "set"/"not set" for each permission (not-set should fall back to global)
	SetBit PermFlag
}

func NewBasePermissions() *BasePermissions {
	return &BasePermissions{0, 0}
}

// Get a permission value. ty should be a power of 2.
// ErrValueNotSet is returned if the permission's set bit is off,
// and should be caught by caller so the global permission can be fetched
func (p *BasePermissions) Get(ty PermFlag) (bool, error) {
	if ty > TopBasePermission {
		return false, ErrInvalidPermission(ty)
	}
	if p.SetBit&ty == 0 {
		return false, ErrValueNotSet(ty)
	}
	return p.Perms&ty > 0, nil
}

// Set a permission bit. Will set the permission's set bit to true.
func (p *BasePermissions) Set(ty PermFlag, value bool) error {
	if ty > TopBasePermission {
		return ErrInvalidPermission(ty)
	}

	p.SetBit |= ty
	if value {
		p.Perms |= ty
	} else {
		p.Perms &= ^ty
	}
	return nil
}

// Set the permission's set bit to false
func (p *BasePermissions) Unset(ty PermFlag) error {
	if ty > TopBasePermission {
		return ErrInvalidPermission(ty)
	}
	p.SetBit &= ^ty
	return nil
}

func (p *BasePermissions) Copy() *BasePermissions {
	return &BasePermissions{
		Perms:  p.Perms,
		SetBit: p.SetBit,
	}
}

func (p *BasePermissions) String() string {
	return fmt.Sprintf("Base: %b; Set: %b", p.Perms, p.SetBit)
}

//---------------------------------------------------------------------------------------------

type AccountPermissions struct {
	Base  *BasePermissions
	Roles []string
}

func NewAccountPermissions() *AccountPermissions {
	return &AccountPermissions{
		Base:  NewBasePermissions(),
		Roles: []string{},
	}
}

// Returns true if the role is found
func (aP *AccountPermissions) HasRole(role string) bool {
	for _, r := range aP.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Returns true if the role is added, and false if it already exists
func (aP *AccountPermissions) AddRole(role string) bool {
	for _, r := range aP.Roles {
		if r == role {
			return false
		}
	}
	aP.Roles = append(aP.Roles, role)
	return true
}

// Returns true if the role is removed, and false if it is not found
func (aP *AccountPermissions) RmRole(role string) bool {
	for i, r := range aP.Roles {
		if r == role {
			post := []string{}
			if len(aP.Roles) > i+1 {
				post = aP.Roles[i+1:]
			}
			aP.Roles = append(aP.Roles[:i], post...)
			return true
		}
	}
	return false
}

func (aP *AccountPermissions) Copy() *AccountPermissions {
	r := make([]string, len(aP.Roles))
	copy(r, aP.Roles)
	return &AccountPermissions{
		Base:  aP.Base.Copy(),
		Roles: r,
	}
}

func NewDefaultAccountPermissions() *AccountPermissions {
	return &AccountPermissions{
		Base: &BasePermissions{
			Perms:  DefaultBBPB,
			SetBit: AllSet,
		},
		Roles: []string{},
	}
}
