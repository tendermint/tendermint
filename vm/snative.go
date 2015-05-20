package vm

import (
	"fmt"

	. "github.com/tendermint/tendermint/common"
	ptypes "github.com/tendermint/tendermint/permission/types"
)

type SNativeContract func(input []byte) (output []byte, err error)

//-----------------------------------------------------------------------------
// snative are native contracts that can access and manipulate the chain state
// (in particular the permissions values)

// TODO: catch errors, log em, return 0s to the vm (should some errors cause exceptions though?)

func (vm *VM) hasBasePerm(args []byte) (output []byte, err error) {
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
	permN := ptypes.PermFlag(Uint64FromWord256(permNum))
	var permInt byte
	if vm.HasPermission(vmAcc, permN) {
		permInt = 0x1
	} else {
		permInt = 0x0
	}
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func (vm *VM) setBasePerm(args []byte) (output []byte, err error) {
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
	permV := !perm.IsZero()
	if err = vmAcc.Permissions.Base.Set(permN, permV); err != nil {
		return nil, err
	}
	vm.appState.UpdateAccount(vmAcc)
	return perm.Bytes(), nil
}

func (vm *VM) unsetBasePerm(args []byte) (output []byte, err error) {
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
	if err = vmAcc.Permissions.Base.Unset(permN); err != nil {
		return nil, err
	}
	vm.appState.UpdateAccount(vmAcc)
	return permNum.Bytes(), nil
}

func (vm *VM) setGlobalPerm(args []byte) (output []byte, err error) {
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
	permV := !perm.IsZero()
	if err = vmAcc.Permissions.Base.Set(permN, permV); err != nil {
		return nil, err
	}
	vm.appState.UpdateAccount(vmAcc)
	return perm.Bytes(), nil
}

// TODO: needs access to an iterator ...
func (vm *VM) clearPerm(args []byte) (output []byte, err error) {
	return nil, nil
}

func (vm *VM) hasRole(args []byte) (output []byte, err error) {
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

func (vm *VM) addRole(args []byte) (output []byte, err error) {
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

func (vm *VM) rmRole(args []byte) (output []byte, err error) {
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
