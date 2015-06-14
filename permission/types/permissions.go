package types

import (
	"fmt"
	. "github.com/tendermint/tendermint/common"
	"reflect"
)

//------------------------------------------------------------------------------------------------

var (
	GlobalPermissionsAddress    = Zero256[:20]
	GlobalPermissionsAddress256 = Zero256
	DougAddress                 = append([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, []byte("THISISDOUG")...)
	DougAddress256              = LeftPadWord256(DougAddress)
)

// A particular permission
type PermFlag uint64

// Base permission references are like unix (the index is already bit shifted)
const (
	Root           PermFlag = 1 << iota // 1
	Send                                // 2
	Call                                // 4
	CreateContract                      // 8
	CreateAccount                       // 16
	Bond                                // 32
	Name                                // 64

	DefaultBBPB = Send | Call | CreateContract | CreateAccount | Bond | Name

	NumBasePermissions uint     = 7
	TopBasePermission  PermFlag = 1 << (NumBasePermissions - 1)
	AllSet             PermFlag = (1 << 63) - 1 + (1 << 63)
)

// should have same ordering as above
type BasePermissionsString struct {
	Root           bool `json:"root,omitempty"`
	Send           bool `json:"send,omitempty"`
	Call           bool `json:"call,omitempty"`
	CreateContract bool `json:"create_contract,omitempty"`
	CreateAccount  bool `json:"create_account,omitempty"`
	Bond           bool `json:"bond,omitempty"`
	Name           bool `json:"name,omitempty"`
}

//---------------------------------------------------------------------------------------------

// Base chain permissions struct
type BasePermissions struct {
	// bit array with "has"/"doesn't have" for each permission
	Perms PermFlag `json:"perms"`

	// bit array with "set"/"not set" for each permission (not-set should fall back to global)
	SetBit PermFlag `json:"set"`
}

func NewBasePermissions() *BasePermissions {
	return &BasePermissions{0, 0}
}

// Get a permission value. ty should be a power of 2.
// ErrValueNotSet is returned if the permission's set bit is off,
// and should be caught by caller so the global permission can be fetched
func (p *BasePermissions) Get(ty PermFlag) (bool, error) {
	if ty == 0 {
		return false, ErrInvalidPermission(ty)
	}
	if p.SetBit&ty == 0 {
		return false, ErrValueNotSet(ty)
	}
	return p.Perms&ty > 0, nil
}

// Set a permission bit. Will set the permission's set bit to true.
func (p *BasePermissions) Set(ty PermFlag, value bool) error {
	if ty == 0 {
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
	if ty == 0 {
		return ErrInvalidPermission(ty)
	}
	p.SetBit &= ^ty
	return nil
}

// Check if the permission is set
func (p *BasePermissions) IsSet(ty PermFlag) bool {
	if ty == 0 {
		return false
	}
	return p.SetBit&ty > 0
}

func (p *BasePermissions) Copy() *BasePermissions {
	if p == nil {
		return nil
	}
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
	Base  *BasePermissions `json:"base"`
	Roles []string         `json:"roles"`
}

func NewAccountPermissions() *AccountPermissions {
	return &AccountPermissions{
		Base:  NewBasePermissions(),
		Roles: []string{},
	}
}

// Returns true if the role is found
func (aP *AccountPermissions) HasRole(role string) bool {
	role = string(LeftPadBytes([]byte(role), 32))
	for _, r := range aP.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Returns true if the role is added, and false if it already exists
func (aP *AccountPermissions) AddRole(role string) bool {
	role = string(LeftPadBytes([]byte(role), 32))
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
	role = string(LeftPadBytes([]byte(role), 32))
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
	if aP == nil {
		return nil
	}
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

//---------------------------------------------------------------------------------------------
// Utilities to make bitmasks human readable

func NewDefaultAccountPermissionsString() BasePermissionsString {
	return BasePermissionsString{
		Root:           false,
		Bond:           true,
		Send:           true,
		Call:           true,
		Name:           true,
		CreateAccount:  true,
		CreateContract: true,
	}
}

func AccountPermissionsFromStrings(perms *BasePermissionsString, roles []string) (*AccountPermissions, error) {
	base := NewBasePermissions()
	permRv := reflect.ValueOf(perms)
	for i := uint(0); i < uint(permRv.NumField()); i++ {
		v := permRv.Field(int(i)).Bool()
		base.Set(1<<i, v)
	}

	aP := &AccountPermissions{
		Base:  base,
		Roles: make([]string, len(roles)),
	}
	copy(aP.Roles, roles)
	return aP, nil
}

func AccountPermissionsToStrings(aP *AccountPermissions) (*BasePermissionsString, []string, error) {
	perms := new(BasePermissionsString)
	permsRv := reflect.ValueOf(perms).Elem()
	for i := uint(0); i < NumBasePermissions; i++ {
		pf := PermFlag(1 << i)
		if aP.Base.IsSet(pf) {
			// won't err if the bit is set
			v, _ := aP.Base.Get(pf)
			f := permsRv.Field(int(i))
			f.SetBool(v)
		}
	}
	roles := make([]string, len(aP.Roles))
	copy(roles, aP.Roles)
	return perms, roles, nil
}

func PermFlagToString(pf PermFlag) (perm string, err error) {
	switch pf {
	case Root:
		perm = "root"
	case Send:
		perm = "send"
	case Call:
		perm = "call"
	case CreateContract:
		perm = "create_contract"
	case CreateAccount:
		perm = "create_account"
	case Bond:
		perm = "bond"
	case Name:
		perm = "name"
	default:
		err = fmt.Errorf("Unknown permission flag %b", pf)
	}
	return
}

func PermStringToFlag(perm string) (pf PermFlag, err error) {
	switch perm {
	case "root":
		pf = Root
	case "send":
		pf = Send
	case "call":
		pf = Call
	case "create_contract":
		pf = CreateContract
	case "create_account":
		pf = CreateAccount
	case "bond":
		pf = Bond
	case "name":
		pf = Name
	default:
		err = fmt.Errorf("Unknown permission %s", perm)
	}
	return
}
