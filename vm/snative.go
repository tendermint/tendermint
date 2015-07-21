package vm

import (
	"fmt"

	. "github.com/tendermint/tendermint/common"
	ptypes "github.com/tendermint/tendermint/permission/types"
)

// TODO: ABI
//------------------------------------------------------------------------------------------------
// Registered SNative contracts

func registerSNativeContracts() {
	registeredNativeContracts[LeftPadWord256([]byte("has_base"))] = hasBasePerm
	registeredNativeContracts[LeftPadWord256([]byte("set_base"))] = setBasePerm
	registeredNativeContracts[LeftPadWord256([]byte("unset_base"))] = unsetBasePerm
	registeredNativeContracts[LeftPadWord256([]byte("set_global"))] = setGlobalPerm
	registeredNativeContracts[LeftPadWord256([]byte("has_role"))] = hasRole
	registeredNativeContracts[LeftPadWord256([]byte("add_role"))] = addRole
	registeredNativeContracts[LeftPadWord256([]byte("rm_role"))] = rmRole
}

//-----------------------------------------------------------------------------
// snative are native contracts that can access and modify an account's permissions

// TODO: catch errors, log em, return 0s to the vm (should some errors cause exceptions though?)

func hasBasePerm(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	if !HasPermission(appState, caller, ptypes.HasBase) {
		return nil, ErrInvalidPermission{caller.Address, "has_base"}
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("hasBasePerm() takes two arguments (address, permFlag)")
	}
	addr, permNum := returnTwoArgs(args)
	vmAcc := appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	permN := ptypes.PermFlag(Uint64FromWord256(permNum)) // already shifted
	if !ValidPermN(permN) {
		return nil, ptypes.ErrInvalidPermission(permN)
	}
	var permInt byte
	if HasPermission(appState, vmAcc, permN) {
		permInt = 0x1
	} else {
		permInt = 0x0
	}
	dbg.Printf("snative.hasBasePerm(0x%X, %b) = %v\n", addr.Postfix(20), permN, permInt)
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func setBasePerm(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	if !HasPermission(appState, caller, ptypes.SetBase) {
		return nil, ErrInvalidPermission{caller.Address, "set_base"}
	}
	if len(args) != 3*32 {
		return nil, fmt.Errorf("setBase() takes three arguments (address, permFlag, permission value)")
	}
	addr, permNum, perm := returnThreeArgs(args)
	vmAcc := appState.GetAccount(addr)
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
	appState.UpdateAccount(vmAcc)
	dbg.Printf("snative.setBasePerm(0x%X, %b, %v)\n", addr.Postfix(20), permN, permV)
	return perm.Bytes(), nil
}

func unsetBasePerm(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	if !HasPermission(appState, caller, ptypes.UnsetBase) {
		return nil, ErrInvalidPermission{caller.Address, "unset_base"}
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("unsetBase() takes two arguments (address, permFlag)")
	}
	addr, permNum := returnTwoArgs(args)
	vmAcc := appState.GetAccount(addr)
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
	appState.UpdateAccount(vmAcc)
	dbg.Printf("snative.unsetBasePerm(0x%X, %b)\n", addr.Postfix(20), permN)
	return permNum.Bytes(), nil
}

func setGlobalPerm(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	if !HasPermission(appState, caller, ptypes.SetGlobal) {
		return nil, ErrInvalidPermission{caller.Address, "set_global"}
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("setGlobal() takes two arguments (permFlag, permission value)")
	}
	permNum, perm := returnTwoArgs(args)
	vmAcc := appState.GetAccount(ptypes.GlobalPermissionsAddress256)
	if vmAcc == nil {
		PanicSanity("cant find the global permissions account")
	}
	permN := ptypes.PermFlag(Uint64FromWord256(permNum))
	if !ValidPermN(permN) {
		return nil, ptypes.ErrInvalidPermission(permN)
	}
	permV := !perm.IsZero()
	if err = vmAcc.Permissions.Base.Set(permN, permV); err != nil {
		return nil, err
	}
	appState.UpdateAccount(vmAcc)
	dbg.Printf("snative.setGlobalPerm(%b, %v)\n", permN, permV)
	return perm.Bytes(), nil
}

func hasRole(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	if !HasPermission(appState, caller, ptypes.HasRole) {
		return nil, ErrInvalidPermission{caller.Address, "has_role"}
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("hasRole() takes two arguments (address, role)")
	}
	addr, role := returnTwoArgs(args)
	vmAcc := appState.GetAccount(addr)
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
	dbg.Printf("snative.hasRole(0x%X, %s) = %v\n", addr.Postfix(20), roleS, permInt > 0)
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func addRole(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	if !HasPermission(appState, caller, ptypes.AddRole) {
		return nil, ErrInvalidPermission{caller.Address, "add_role"}
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("addRole() takes two arguments (address, role)")
	}
	addr, role := returnTwoArgs(args)
	vmAcc := appState.GetAccount(addr)
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
	appState.UpdateAccount(vmAcc)
	dbg.Printf("snative.addRole(0x%X, %s) = %v\n", addr.Postfix(20), roleS, permInt > 0)
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func rmRole(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	if !HasPermission(appState, caller, ptypes.RmRole) {
		return nil, ErrInvalidPermission{caller.Address, "rm_role"}
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("rmRole() takes two arguments (address, role)")
	}
	addr, role := returnTwoArgs(args)
	vmAcc := appState.GetAccount(addr)
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
	appState.UpdateAccount(vmAcc)
	dbg.Printf("snative.rmRole(0x%X, %s) = %v\n", addr.Postfix(20), roleS, permInt > 0)
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

//------------------------------------------------------------------------------------------------
// Errors and utility funcs

type ErrInvalidPermission struct {
	Address Word256
	SNative string
}

func (e ErrInvalidPermission) Error() string {
	return fmt.Sprintf("Account %X does not have permission snative.%s", e.Address.Postfix(20), e.SNative)
}

// Checks if a permission flag is valid (a known base chain or snative permission)
func ValidPermN(n ptypes.PermFlag) bool {
	if n > ptypes.TopPermFlag {
		return false
	}
	return true
}

// CONTRACT: length has already been checked
func returnTwoArgs(args []byte) (a Word256, b Word256) {
	copy(a[:], args[:32])
	copy(b[:], args[32:64])
	return
}

// CONTRACT: length has already been checked
func returnThreeArgs(args []byte) (a Word256, b Word256, c Word256) {
	copy(a[:], args[:32])
	copy(b[:], args[32:64])
	copy(c[:], args[64:96])
	return
}
