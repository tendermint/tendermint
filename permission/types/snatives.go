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

// we need to reset iota with no const block
const (
	// each snative has an associated permission flag
	HasBase PermFlag = FirstSNativePermFlag << iota
	SetBase
	UnsetBase
	SetGlobal
	ClearBase
	HasRole
	AddRole
	RmRole
	NumSNativePermissions uint = 8 // NOTE adjust this too

	TopSNativePermFlag  PermFlag = FirstSNativePermFlag << (NumSNativePermissions - 1)
	AllSNativePermFlags PermFlag = (TopSNativePermFlag | (TopSNativePermFlag - 1)) &^ (FirstSNativePermFlag - 1)
)

//---------------------------------------------------------------------------------------------------
// snative tx interface and argument encoding

// SNatives are represented as an interface in the SNative tx,
// so binary does the work of handling args and determining which snative we're calling
// NOTE: we're definining an implicit map from registration bytes to perm flags
type SNativeArgs interface {
	PermFlag() PermFlag
}

const (
	SNativeArgsTypeHasBase   = byte(0x01)
	SNativeArgsTypeSetBase   = byte(0x02)
	SNativeArgsTypeUnsetBase = byte(0x03)
	SNativeArgsTypeSetGlobal = byte(0x04)
	SNativeArgsTypeClearBase = byte(0x05)
	SNativeArgsTypeHasRole   = byte(0x06)
	SNativeArgsTypeAddRole   = byte(0x07)
	SNativeArgsTypeRmRole    = byte(0x08)
)

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ SNativeArgs }{},
	binary.ConcreteType{&HasBaseArgs{}, SNativeArgsTypeHasBase},
	binary.ConcreteType{&SetBaseArgs{}, SNativeArgsTypeSetBase},
	binary.ConcreteType{&UnsetBaseArgs{}, SNativeArgsTypeUnsetBase},
	binary.ConcreteType{&SetGlobalArgs{}, SNativeArgsTypeSetGlobal},
	binary.ConcreteType{&ClearBaseArgs{}, SNativeArgsTypeClearBase},
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

type ClearBaseArgs struct {
	//
}

func (*ClearBaseArgs) PermFlag() PermFlag {
	return ClearBase
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
	case ClearBase:
		perm = "ClearBase"
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
	case "ClearBase":
		pF = ClearBase
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
