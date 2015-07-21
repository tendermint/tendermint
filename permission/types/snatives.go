package types

import (
	"fmt"

	"github.com/tendermint/tendermint/binary"
)

//---------------------------------------------------------------------------------------------------
// snative permissions

const (
	// first 32 bits of BasePermission are for chain, second 32 are for snative
	FirstSNativePermFlag PermFlag = 1 << 32
)

// we need to reset iota with new const block
const (
	// each snative has an associated permission flag
	HasBase PermFlag = FirstSNativePermFlag << iota
	SetBase
	UnsetBase
	SetGlobal
	HasRole
	AddRole
	RmRole
	NumSNativePermissions uint = 7 // NOTE adjust this too

	TopSNativePermFlag  PermFlag = FirstSNativePermFlag << (NumSNativePermissions - 1)
	AllSNativePermFlags PermFlag = (TopSNativePermFlag | (TopSNativePermFlag - 1)) &^ (FirstSNativePermFlag - 1)
)

//---------------------------------------------------------------------------------------------------
// snative tx interface and argument encoding

// SNativesArgs are a registered interface in the SNativeTx,
// so binary handles the arguments and each snative gets a type-byte
// PermFlag() maps the type-byte to the permission
// The account sending the SNativeTx must have this PermFlag set
type SNativeArgs interface {
	PermFlag() PermFlag
}

const (
	SNativeArgsTypeHasBase   = byte(0x01)
	SNativeArgsTypeSetBase   = byte(0x02)
	SNativeArgsTypeUnsetBase = byte(0x03)
	SNativeArgsTypeSetGlobal = byte(0x04)
	SNativeArgsTypeHasRole   = byte(0x05)
	SNativeArgsTypeAddRole   = byte(0x06)
	SNativeArgsTypeRmRole    = byte(0x07)
)

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ SNativeArgs }{},
	binary.ConcreteType{&HasBaseArgs{}, SNativeArgsTypeHasBase},
	binary.ConcreteType{&SetBaseArgs{}, SNativeArgsTypeSetBase},
	binary.ConcreteType{&UnsetBaseArgs{}, SNativeArgsTypeUnsetBase},
	binary.ConcreteType{&SetGlobalArgs{}, SNativeArgsTypeSetGlobal},
	binary.ConcreteType{&HasRoleArgs{}, SNativeArgsTypeHasRole},
	binary.ConcreteType{&AddRoleArgs{}, SNativeArgsTypeAddRole},
	binary.ConcreteType{&RmRoleArgs{}, SNativeArgsTypeRmRole},
)

type HasBaseArgs struct {
	Address    []byte
	Permission PermFlag
}

func (*HasBaseArgs) PermFlag() PermFlag {
	return HasBase
}

type SetBaseArgs struct {
	Address    []byte
	Permission PermFlag
	Value      bool
}

func (*SetBaseArgs) PermFlag() PermFlag {
	return SetBase
}

type UnsetBaseArgs struct {
	Address    []byte
	Permission PermFlag
}

func (*UnsetBaseArgs) PermFlag() PermFlag {
	return UnsetBase
}

type SetGlobalArgs struct {
	Permission PermFlag
	Value      bool
}

func (*SetGlobalArgs) PermFlag() PermFlag {
	return SetGlobal
}

type HasRoleArgs struct {
	Address []byte
	Role    string
}

func (*HasRoleArgs) PermFlag() PermFlag {
	return HasRole
}

type AddRoleArgs struct {
	Address []byte
	Role    string
}

func (*AddRoleArgs) PermFlag() PermFlag {
	return AddRole
}

type RmRoleArgs struct {
	Address []byte
	Role    string
}

func (*RmRoleArgs) PermFlag() PermFlag {
	return RmRole
}

//------------------------------------------------------------
// string utilities

func SNativePermFlagToString(pF PermFlag) (perm string) {
	switch pF {
	case HasBase:
		perm = "HasBase"
	case SetBase:
		perm = "SetBase"
	case UnsetBase:
		perm = "UnsetBase"
	case SetGlobal:
		perm = "SetGlobal"
	case HasRole:
		perm = "HasRole"
	case AddRole:
		perm = "AddRole"
	case RmRole:
		perm = "RmRole"
	default:
		perm = "#-UNKNOWN-#"
	}
	return
}

func SNativeStringToPermFlag(perm string) (pF PermFlag, err error) {
	switch perm {
	case "HasBase":
		pF = HasBase
	case "SetBase":
		pF = SetBase
	case "UnsetBase":
		pF = UnsetBase
	case "SetGlobal":
		pF = SetGlobal
	case "HasRole":
		pF = HasRole
	case "AddRole":
		pF = AddRole
	case "RmRole":
		pF = RmRole
	default:
		err = fmt.Errorf("Unknown permission %s", perm)
	}
	return
}
