package vm

import (
	"fmt"

	. "github.com/tendermint/tendermint/common"
	ptypes "github.com/tendermint/tendermint/permission/types"
)

// Checks if a permission flag is valid (a known base chain or snative permission)
func ValidPermN(n ptypes.PermFlag) bool {
	if n > ptypes.TopBasePermission && n < FirstSNativePerm {
		return false
	} else if n > TopSNativePermission {
		return false
	}
	return true
}

const (
	// first 32 bits of BasePermission are for chain, second 32 are for snative
	FirstSNativePerm ptypes.PermFlag = 1 << 32

	// each snative has an associated permission flag
	HasBasePerm ptypes.PermFlag = FirstSNativePerm << iota
	SetBasePerm
	UnsetBasePerm
	SetGlobalPerm
	ClearBasePerm
	HasRole
	AddRole
	RmRole

	// XXX: must be adjusted if snative's added/removed
	NumSNativePermissions uint            = 8
	TopSNativePermission  ptypes.PermFlag = FirstSNativePerm << (NumSNativePermissions - 1)
)

var registeredSNativeContracts = map[Word256]ptypes.PermFlag{
	RightPadWord256([]byte("hasBasePerm")):   HasBasePerm,
	RightPadWord256([]byte("setBasePerm")):   SetBasePerm,
	RightPadWord256([]byte("unsetBasePerm")): UnsetBasePerm,
	RightPadWord256([]byte("setGlobalPerm")): SetGlobalPerm,
	RightPadWord256([]byte("hasRole")):       HasRole,
	RightPadWord256([]byte("addRole")):       AddRole,
	RightPadWord256([]byte("rmRole")):        RmRole,
}

// takes an account so it can check for permission to access the contract
// NOTE: the account is the currently executing account (the callee), not say an origin caller
type SNativeContract func(acc *Account, input []byte) (output []byte, err error)

func (vm *VM) SNativeContract(name Word256) SNativeContract {
	flag := registeredSNativeContracts[name]
	switch flag {
	case HasBasePerm:
		return vm.hasBasePerm
	case SetBasePerm:
		return vm.setBasePerm
	case UnsetBasePerm:
		return vm.unsetBasePerm
	case SetGlobalPerm:
		return vm.setGlobalPerm
	case HasRole:
		return vm.hasRole
	case AddRole:
		return vm.addRole
	case RmRole:
		return vm.rmRole
	default:
		return nil
	}
}

//-----------------------------------------------------------------------------
// snative are native contracts that can access and manipulate the chain state
// (in particular the permissions values)

// TODO: catch errors, log em, return 0s to the vm (should some errors cause exceptions though?)

func (vm *VM) hasBasePerm(acc *Account, args []byte) (output []byte, err error) {
	if !vm.HasPermission(acc, HasBasePerm) {
		return nil, fmt.Errorf("acc %X does not have permission to call snative.HasBasePerm")
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("hasBasePerm() takes two arguments (address, permission number)")
	}
	var addr, permNum Word256
	copy(addr[:], args[:32])
	copy(permNum[:], args[32:64])
	vmAcc := vm.appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	permN := ptypes.PermFlag(Uint64FromWord256(permNum)) // already shifted
	if !ValidPermN(permN) {
		return nil, ptypes.ErrInvalidPermission(permN)
	}
	var permInt byte
	if vm.HasPermission(vmAcc, permN) {
		permInt = 0x1
	} else {
		permInt = 0x0
	}
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func (vm *VM) setBasePerm(acc *Account, args []byte) (output []byte, err error) {
	if !vm.HasPermission(acc, SetBasePerm) {
		return nil, fmt.Errorf("acc %X does not have permission to call snative.SetBasePerm")
	}
	if len(args) != 3*32 {
		return nil, fmt.Errorf("setBasePerm() takes three arguments (address, permission number, permission value)")
	}
	var addr, permNum, perm Word256
	copy(addr[:], args[:32])
	copy(permNum[:], args[32:64])
	copy(perm[:], args[64:96])
	vmAcc := vm.appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	permN := ptypes.PermFlag(Uint64FromWord256(permNum))
	if !ValidPermN(permN) {
		return nil, ptypes.ErrInvalidPermission(permN)
	}
	permV := !perm.IsZero()
	if err = vmAcc.Permissions.Base.Set(permN, permV); err != nil {
		return nil, err
	}
	vm.appState.UpdateAccount(vmAcc)
	return perm.Bytes(), nil
}

func (vm *VM) unsetBasePerm(acc *Account, args []byte) (output []byte, err error) {
	if !vm.HasPermission(acc, UnsetBasePerm) {
		return nil, fmt.Errorf("acc %X does not have permission to call snative.UnsetBasePerm")
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("unsetBasePerm() takes two arguments (address, permission number)")
	}
	var addr, permNum Word256
	copy(addr[:], args[:32])
	copy(permNum[:], args[32:64])
	vmAcc := vm.appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	permN := ptypes.PermFlag(Uint64FromWord256(permNum))
	if !ValidPermN(permN) {
		return nil, ptypes.ErrInvalidPermission(permN)
	}
	if err = vmAcc.Permissions.Base.Unset(permN); err != nil {
		return nil, err
	}
	vm.appState.UpdateAccount(vmAcc)
	return permNum.Bytes(), nil
}

func (vm *VM) setGlobalPerm(acc *Account, args []byte) (output []byte, err error) {
	if len(args) != 2*32 {
		return nil, fmt.Errorf("setGlobalPerm() takes three arguments (permission number, permission value)")
	}
	var permNum, perm Word256
	copy(permNum[:], args[32:64])
	copy(perm[:], args[64:96])
	vmAcc := vm.appState.GetAccount(ptypes.GlobalPermissionsAddress256)
	if vmAcc == nil {
		panic("cant find the global permissions account")
	}
	permN := ptypes.PermFlag(Uint64FromWord256(permNum))
	if !ValidPermN(permN) {
		return nil, ptypes.ErrInvalidPermission(permN)
	}
	permV := !perm.IsZero()
	if err = vmAcc.Permissions.Base.Set(permN, permV); err != nil {
		return nil, err
	}
	vm.appState.UpdateAccount(vmAcc)
	return perm.Bytes(), nil
}

// TODO: needs access to an iterator ...
func (vm *VM) clearPerm(acc *Account, args []byte) (output []byte, err error) {
	if !vm.HasPermission(acc, ClearBasePerm) {
		return nil, fmt.Errorf("acc %X does not have permission to call snative.ClearBasePerm")
	}
	return nil, nil
}

func (vm *VM) hasRole(acc *Account, args []byte) (output []byte, err error) {
	if !vm.HasPermission(acc, HasRole) {
		return nil, fmt.Errorf("acc %X does not have permission to call snative.HasRole")
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("hasRole() takes two arguments (address, role)")
	}
	var addr, role Word256
	copy(addr[:], args[32:64])
	copy(role[:], args[64:96])
	vmAcc := vm.appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	roleS := string(role.Bytes())
	var permInt byte
	if vmAcc.Permissions.HasRole(roleS) {
		permInt = 0x1
	} else {
		permInt = 0x0
	}
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func (vm *VM) addRole(acc *Account, args []byte) (output []byte, err error) {
	if !vm.HasPermission(acc, AddRole) {
		return nil, fmt.Errorf("acc %X does not have permission to call snative.AddRole")
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("addRole() takes two arguments (address, role)")
	}
	var addr, role Word256
	copy(addr[:], args[32:64])
	copy(role[:], args[64:96])
	vmAcc := vm.appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	roleS := string(role.Bytes())
	var permInt byte
	if vmAcc.Permissions.AddRole(roleS) {
		permInt = 0x1
	} else {
		permInt = 0x0
	}
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func (vm *VM) rmRole(acc *Account, args []byte) (output []byte, err error) {
	if !vm.HasPermission(acc, RmRole) {
		return nil, fmt.Errorf("acc %X does not have permission to call snative.RmRole")
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("rmRole() takes two arguments (address, role)")
	}
	var addr, role Word256
	copy(addr[:], args[32:64])
	copy(role[:], args[64:96])
	vmAcc := vm.appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	roleS := string(role.Bytes())
	var permInt byte
	if vmAcc.Permissions.RmRole(roleS) {
		permInt = 0x1
	} else {
		permInt = 0x0
	}
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}
