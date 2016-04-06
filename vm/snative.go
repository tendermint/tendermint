package vm

import (
	"encoding/hex"
	"fmt"

	. "github.com/eris-ltd/tendermint/common"
	ptypes "github.com/eris-ltd/tendermint/permission/types"
)

//------------------------------------------------------------------------------------------------
// Registered SNative contracts

var PermissionsContract = "permissions_contract"

func registerSNativeContracts() {
	registeredNativeContracts[LeftPadWord256([]byte(PermissionsContract))] = permissionsContract

	/*
		// we could expose these but we moved permission and args checks into the permissionsContract
		// so calling them would be unsafe ...
		registeredNativeContracts[LeftPadWord256([]byte("has_base"))] = has_base
		registeredNativeContracts[LeftPadWord256([]byte("set_base"))] = set_base
		registeredNativeContracts[LeftPadWord256([]byte("unset_base"))] = unset_base
		registeredNativeContracts[LeftPadWord256([]byte("set_global"))] = set_global
		registeredNativeContracts[LeftPadWord256([]byte("has_role"))] = has_role
		registeredNativeContracts[LeftPadWord256([]byte("add_role"))] = add_role
		registeredNativeContracts[LeftPadWord256([]byte("rm_role"))] = rm_role
	*/
}

//-----------------------------------------------------------------------------
// snative are native contracts that can access and modify an account's permissions

type SNativeFuncDescription struct {
	Name     string
	NArgs    int
	PermFlag ptypes.PermFlag
	F        NativeContract
}

/* The solidity interface used to generate the abi function ids below
contract Permissions {
	function has_base(address addr, int permFlag) constant returns (bool value) {}
	function set_base(address addr, int permFlag, bool value) constant returns (bool val) {}
	function unset_base(address addr, int permFlag) constant returns (int pf) {}
	function set_global(address addr, int permFlag, bool value) constant returns (int pf) {}
	function has_role(address addr, string role) constant returns (bool val) {}
	function add_role(address addr, string role) constant returns (bool added) {}
	function rm_role(address addr, string role) constant returns (bool removed) {}
}
*/

// function identifiers from the solidity abi
var PermsMap = map[string]SNativeFuncDescription{
	"054556ac": SNativeFuncDescription{"has_role", 2, ptypes.HasRole, has_role},
	"180d26f2": SNativeFuncDescription{"unset_base", 2, ptypes.UnsetBase, unset_base},
	"3a3fcc59": SNativeFuncDescription{"set_global", 2, ptypes.SetGlobal, set_global},
	"9a1c4141": SNativeFuncDescription{"add_role", 2, ptypes.AddRole, add_role},
	"9ea53314": SNativeFuncDescription{"set_base", 3, ptypes.SetBase, set_base},
	"bb37737a": SNativeFuncDescription{"has_base", 2, ptypes.HasBase, has_base},
	"ded3350a": SNativeFuncDescription{"rm_role", 2, ptypes.RmRole, rm_role},
}

func permissionsContract(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("permissionsContract expects at least a 4-byte function identifier")
	}

	// map solidity abi function id to snative
	funcIDbytes := args[:4]
	args = args[4:]
	funcID := hex.EncodeToString(funcIDbytes)
	d, ok := PermsMap[funcID]
	if !ok {
		return nil, fmt.Errorf("unknown permissionsContract funcID %s", funcID)
	}

	// check if we have permission to call this function
	if !HasPermission(appState, caller, d.PermFlag) {
		return nil, ErrInvalidPermission{caller.Address, d.Name}
	}

	// ensure there are enough arguments
	if len(args) != d.NArgs*32 {
		return nil, fmt.Errorf("%s() takes %d arguments", d.Name)
	}

	// call the function
	return d.F(appState, caller, args, gas)
}

// TODO: catch errors, log em, return 0s to the vm (should some errors cause exceptions though?)

func has_base(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	addr, permNum := returnTwoArgs(args)
	vmAcc := appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	permN := ptypes.PermFlag(Uint64FromWord256(permNum)) // already shifted
	if !ValidPermN(permN) {
		return nil, ptypes.ErrInvalidPermission(permN)
	}
	permInt := byteFromBool(HasPermission(appState, vmAcc, permN))
	dbg.Printf("snative.hasBasePerm(0x%X, %b) = %v\n", addr.Postfix(20), permN, permInt)
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func set_base(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
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

func unset_base(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
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

func set_global(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
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

func has_role(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	addr, role := returnTwoArgs(args)
	vmAcc := appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	roleS := string(role.Bytes())
	permInt := byteFromBool(vmAcc.Permissions.HasRole(roleS))
	dbg.Printf("snative.hasRole(0x%X, %s) = %v\n", addr.Postfix(20), roleS, permInt > 0)
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func add_role(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	addr, role := returnTwoArgs(args)
	vmAcc := appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	roleS := string(role.Bytes())
	permInt := byteFromBool(vmAcc.Permissions.AddRole(roleS))
	appState.UpdateAccount(vmAcc)
	dbg.Printf("snative.addRole(0x%X, %s) = %v\n", addr.Postfix(20), roleS, permInt > 0)
	return LeftPadWord256([]byte{permInt}).Bytes(), nil
}

func rm_role(appState AppState, caller *Account, args []byte, gas *int64) (output []byte, err error) {
	addr, role := returnTwoArgs(args)
	vmAcc := appState.GetAccount(addr)
	if vmAcc == nil {
		return nil, fmt.Errorf("Unknown account %X", addr)
	}
	roleS := string(role.Bytes())
	permInt := byteFromBool(vmAcc.Permissions.RmRole(roleS))
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

func byteFromBool(b bool) byte {
	if b {
		return 0x1
	}
	return 0x0
}
