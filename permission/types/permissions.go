package types

import (
	"fmt"
	. "github.com/tendermint/tendermint/common"
)

//------------------------------------------------------------------------------------------------

var (
	GlobalPermissionsAddress    = Zero256[:20]
	GlobalPermissionsAddress256 = Zero256
)

// A particular permission
type PermFlag uint64

// Base permission references are like unix (the index is already bit shifted)
const (
	// chain permissions
	Root           PermFlag = 1 << iota // 1
	Send                                // 2
	Call                                // 4
	CreateContract                      // 8
	CreateAccount                       // 16
	Bond                                // 32
	Name                                // 64

	// moderator permissions
	HasBase
	SetBase
	UnsetBase
	SetGlobal
	HasRole
	AddRole
	RmRole

	NumPermissions uint = 14 // NOTE Adjust this too. We can support upto 64

	TopPermFlag      PermFlag = 1 << (NumPermissions - 1)
	AllPermFlags     PermFlag = TopPermFlag | (TopPermFlag - 1)
	DefaultPermFlags PermFlag = Send | Call | CreateContract | CreateAccount | Bond | Name | HasBase | HasRole
)

var (
	ZeroBasePermissions    = BasePermissions{0, 0}
	ZeroAccountPermissions = AccountPermissions{
		Base: ZeroBasePermissions,
	}
	DefaultAccountPermissions = AccountPermissions{
		Base: BasePermissions{
			Perms:  DefaultPermFlags,
			SetBit: AllPermFlags,
		},
		Roles: []string{},
	}
)

//---------------------------------------------------------------------------------------------

// Base chain permissions struct
type BasePermissions struct {
	// bit array with "has"/"doesn't have" for each permission
	Perms PermFlag `json:"perms"`

	// bit array with "set"/"not set" for each permission (not-set should fall back to global)
	SetBit PermFlag `json:"set"`
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

func (p BasePermissions) String() string {
	return fmt.Sprintf("Base: %b; Set: %b", p.Perms, p.SetBit)
}

//---------------------------------------------------------------------------------------------

type AccountPermissions struct {
	Base  BasePermissions `json:"base"`
	Roles []string        `json:"roles"`
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

//--------------------------------------------------------------------------------
// string utilities

// PermFlagToString assumes the permFlag is valid, else returns "#-UNKNOWN-#"
func PermFlagToString(pf PermFlag) (perm string) {
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
	case HasBase:
		perm = "has_base"
	case SetBase:
		perm = "set_base"
	case UnsetBase:
		perm = "unset_base"
	case SetGlobal:
		perm = "set_global"
	case HasRole:
		perm = "has_role"
	case AddRole:
		perm = "add_role"
	case RmRole:
		perm = "rm_role"
	default:
		perm = "#-UNKNOWN-#"
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
	case "has_base":
		pf = HasBase
	case "set_base":
		pf = SetBase
	case "unset_base":
		pf = UnsetBase
	case "set_global":
		pf = SetGlobal
	case "has_role":
		pf = HasRole
	case "add_role":
		pf = AddRole
	case "rm_role":
		pf = RmRole
	default:
		err = fmt.Errorf("Unknown permission %s", perm)
	}
	return
}
