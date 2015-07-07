package vm

import (
	"fmt"

	. "github.com/tendermint/tendermint/common"
	ptypes "github.com/tendermint/tendermint/permission/types"
)

//------------------------------------------------------------------------------------------------
// Registered SNative contracts

var RegisteredSNativeContracts = map[Word256]SNativeContract{
	LeftPadWord256([]byte("hasBasePerm")):   hasBasePerm,
	LeftPadWord256([]byte("setBasePerm")):   setBasePerm,
	LeftPadWord256([]byte("unsetBasePerm")): unsetBasePerm,
	LeftPadWord256([]byte("setGlobalPerm")): setGlobalPerm,
	LeftPadWord256([]byte("hasRole")):       hasRole,
	LeftPadWord256([]byte("addRole")):       addRole,
	LeftPadWord256([]byte("rmRole")):        rmRole,
}

// Takes an appState so it can lookup/update accounts,
// an account to check for permission to access the snative contract
// and some input bytes (presumably 32byte words)
type SNativeContract func(appState AppState, acc *Account, input []byte) (output []byte, err error)

//-----------------------------------------------------------------------------
// snative are native contracts that can access and manipulate the chain state
// (in particular the permissions values)

// TODO: catch errors, log em, return 0s to the vm (should some errors cause exceptions though?)

func hasBasePerm(appState AppState, acc *Account, args []byte) (output []byte, err error) {
	if !HasPermission(appState, acc, ptypes.HasBasePerm) {
		return nil, ErrInvalidPermission{acc.Address, "HasBasePerm"}
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("hasBasePerm() takes two arguments (address, permission number)")
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

func setBasePerm(appState AppState, acc *Account, args []byte) (output []byte, err error) {
	if !HasPermission(appState, acc, ptypes.SetBasePerm) {
		return nil, ErrInvalidPermission{acc.Address, "SetBasePerm"}
	}
	if len(args) != 3*32 {
		return nil, fmt.Errorf("setBasePerm() takes three arguments (address, permission number, permission value)")
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

func unsetBasePerm(appState AppState, acc *Account, args []byte) (output []byte, err error) {
	if !HasPermission(appState, acc, ptypes.UnsetBasePerm) {
		return nil, ErrInvalidPermission{acc.Address, "UnsetBasePerm"}
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("unsetBasePerm() takes two arguments (address, permission number)")
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

func setGlobalPerm(appState AppState, acc *Account, args []byte) (output []byte, err error) {
	if !HasPermission(appState, acc, ptypes.SetGlobalPerm) {
		return nil, ErrInvalidPermission{acc.Address, "SetGlobalPerm"}
	}
	if len(args) != 2*32 {
		return nil, fmt.Errorf("setGlobalPerm() takes two arguments (permission number, permission value)")
	}
	permNum, perm := returnTwoArgs(args)
	vmAcc := appState.GetAccount(ptypes.GlobalPermissionsAddress256)
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
	appState.UpdateAccount(vmAcc)
	dbg.Printf("snative.setGlobalPerm(%b, %v)\n", permN, permV)
	return perm.Bytes(), nil
}

// TODO: needs access to an iterator ...
func clearPerm(appState AppState, acc *Account, args []byte) (output []byte, err error) {
	if !HasPermission(appState, acc, ptypes.ClearBasePerm) {
		return nil, ErrInvalidPermission{acc.Address, "ClearPerm"}
	}
	return nil, nil
}

func hasRole(appState AppState, acc *Account, args []byte) (output []byte, err error) {
	if !HasPermission(appState, acc, ptypes.HasRole) {
		return nil, ErrInvalidPermission{acc.Address, "HasRole"}
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

func addRole(appState AppState, acc *Account, args []byte) (output []byte, err error) {
	if !HasPermission(appState, acc, ptypes.AddRole) {
		return nil, ErrInvalidPermission{acc.Address, "AddRole"}
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

func rmRole(appState AppState, acc *Account, args []byte) (output []byte, err error) {
	if !HasPermission(appState, acc, ptypes.RmRole) {
		return nil, ErrInvalidPermission{acc.Address, "RmRole"}
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
	if n > ptypes.TopBasePermFlag && n < ptypes.FirstSNativePermFlag {
		return false
	} else if n > ptypes.TopSNativePermFlag {
		return false
	}
	return true
}

// assumes length has already been checked
func returnTwoArgs(args []byte) (a Word256, b Word256) {
	copy(a[:], args[:32])
	copy(b[:], args[32:64])
	return
}

// assumes length has already been checked
func returnThreeArgs(args []byte) (a Word256, b Word256, c Word256) {
	copy(a[:], args[:32])
	copy(b[:], args[32:64])
	copy(c[:], args[64:96])
	return
}

// mostly a convenience for testing
var RegisteredSNativePermissions = map[Word256]ptypes.PermFlag{
	LeftPadWord256([]byte("hasBasePerm")):   ptypes.HasBasePerm,
	LeftPadWord256([]byte("setBasePerm")):   ptypes.SetBasePerm,
	LeftPadWord256([]byte("unsetBasePerm")): ptypes.UnsetBasePerm,
	LeftPadWord256([]byte("setGlobalPerm")): ptypes.SetGlobalPerm,
	LeftPadWord256([]byte("hasRole")):       ptypes.HasRole,
	LeftPadWord256([]byte("addRole")):       ptypes.AddRole,
	LeftPadWord256([]byte("rmRole")):        ptypes.RmRole,
}
