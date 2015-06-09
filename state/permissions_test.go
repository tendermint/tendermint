package state

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/events"
	ptypes "github.com/tendermint/tendermint/permission/types"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/vm"
)

/*
Permission Tests:

- SendTx:
x	- 1 input, no perm, call perm, create perm
x	- 1 input, perm
x	- 2 inputs, one with perm one without

- CallTx, CALL
x	- 1 input, no perm, send perm, create perm
x	- 1 input, perm
x	- contract runs call but doesn't have call perm
x	- contract runs call and has call perm
x	- contract runs call (with perm), runs contract that runs call (without perm)
x	- contract runs call (with perm), runs contract that runs call (with perm)

- CallTx for Create, CREATE
x	- 1 input, no perm, send perm, call perm
x 	- 1 input, perm
x	- contract runs create but doesn't have create perm
x	- contract runs create but has perm
x	- contract runs call with empty address (has call and create perm)

- NameTx
	- no perm, send perm, call perm
	- with perm

- BondTx
x	- 1 input, no perm
x	- 1 input, perm
x	- 1 bonder with perm, input without send or bond
x	- 1 bonder with perm, input with send
x	- 1 bonder with perm, input with bond
x	- 2 inputs, one with perm one without

- SendTx for new account
x 	- 1 input, 1 unknown ouput, input with send, not create  (fail)
x 	- 1 input, 1 unknown ouput, input with send and create (pass)
x 	- 2 inputs, 1 unknown ouput, both inputs with send, one with create, one without (fail)
x 	- 2 inputs, 1 known output, 1 unknown ouput, one input with create, one without (fail)
x 	- 2 inputs, 1 unknown ouput, both inputs with send, both inputs with create (pass )
x 	- 2 inputs, 1 known output, 1 unknown ouput, both inputs with create, (pass)


- CALL for new account
x	- unknown output, without create (fail)
x	- unknown output, with create (pass)


- SNative (CallTx, CALL):
	- for each of CallTx, Call
x		- call each snative without permission, fails
x		- call each snative with permission, pass
	- list:
x		- base: has,set,unset
x		- globals: set
x 		- roles: has, add, rm


*/

// keys
var user = makeUsers(10)
var chainID = "testchain"

func makeUsers(n int) []*account.PrivAccount {
	accounts := []*account.PrivAccount{}
	for i := 0; i < n; i++ {
		secret := []byte("mysecret" + strconv.Itoa(i))
		user := account.GenPrivAccountFromSecret(secret)
		accounts = append(accounts, user)
	}
	return accounts
}

var (
	PermsAllFalse = ptypes.NewAccountPermissions()
)

func newBaseGenDoc(globalPerm, accountPerm *ptypes.AccountPermissions) GenesisDoc {
	genAccounts := []GenesisAccount{}
	for _, u := range user[:5] {
		genAccounts = append(genAccounts, GenesisAccount{
			Address:     u.Address,
			Amount:      1000000,
			Permissions: accountPerm.Copy(),
		})
	}

	return GenesisDoc{
		GenesisTime: time.Now(),
		ChainID:     chainID,
		Params: &GenesisParams{
			GlobalPermissions: globalPerm,
		},
		Accounts: genAccounts,
		Validators: []GenesisValidator{
			GenesisValidator{
				PubKey: user[0].PubKey.(account.PubKeyEd25519),
				Amount: 10,
				UnbondTo: []GenesisAccount{
					GenesisAccount{
						Address: user[0].Address,
					},
				},
			},
		},
	}
}

func TestSendFails(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[1].Permissions.Base.Set(ptypes.Send, true)
	genDoc.Accounts[2].Permissions.Base.Set(ptypes.Call, true)
	genDoc.Accounts[3].Permissions.Base.Set(ptypes.CreateContract, true)
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//-------------------
	// send txs

	// simple send tx should fail
	tx := types.NewSendTx()
	if err := tx.AddInput(blockCache, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[1].Address, 5)
	tx.SignInput(chainID, 0, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple send tx with call perm should fail
	tx = types.NewSendTx()
	if err := tx.AddInput(blockCache, user[2].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[4].Address, 5)
	tx.SignInput(chainID, 0, user[2])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple send tx with create perm should fail
	tx = types.NewSendTx()
	if err := tx.AddInput(blockCache, user[3].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[4].Address, 5)
	tx.SignInput(chainID, 0, user[3])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple send tx to unknown account without create_account perm should fail
	acc := blockCache.GetAccount(user[3].Address)
	acc.Permissions.Base.Set(ptypes.Send, true)
	blockCache.UpdateAccount(acc)
	tx = types.NewSendTx()
	if err := tx.AddInput(blockCache, user[3].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[6].Address, 5)
	tx.SignInput(chainID, 0, user[3])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}
}

func TestName(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Base.Set(ptypes.Send, true)
	genDoc.Accounts[1].Permissions.Base.Set(ptypes.Name, true)
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//-------------------
	// name txs

	// simple name tx without perm should fail
	tx, err := types.NewNameTx(st, user[0].PubKey, "somename", "somedata", 10000, 100)
	if err != nil {
		t.Fatal(err)
	}
	tx.Sign(chainID, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple name tx with perm should pass
	tx, err = types.NewNameTx(st, user[1].PubKey, "somename", "somedata", 10000, 100)
	if err != nil {
		t.Fatal(err)
	}
	tx.Sign(chainID, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCallFails(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[1].Permissions.Base.Set(ptypes.Send, true)
	genDoc.Accounts[2].Permissions.Base.Set(ptypes.Call, true)
	genDoc.Accounts[3].Permissions.Base.Set(ptypes.CreateContract, true)
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//-------------------
	// call txs

	// simple call tx should fail
	tx, _ := types.NewCallTx(blockCache, user[0].PubKey, user[4].Address, nil, 100, 100, 100)
	tx.Sign(chainID, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple call tx with send permission should fail
	tx, _ = types.NewCallTx(blockCache, user[1].PubKey, user[4].Address, nil, 100, 100, 100)
	tx.Sign(chainID, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple call tx with create permission should fail
	tx, _ = types.NewCallTx(blockCache, user[3].PubKey, user[4].Address, nil, 100, 100, 100)
	tx.Sign(chainID, user[3])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	//-------------------
	// create txs

	// simple call create tx should fail
	tx, _ = types.NewCallTx(blockCache, user[0].PubKey, nil, nil, 100, 100, 100)
	tx.Sign(chainID, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple call create tx with send perm should fail
	tx, _ = types.NewCallTx(blockCache, user[1].PubKey, nil, nil, 100, 100, 100)
	tx.Sign(chainID, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple call create tx with call perm should fail
	tx, _ = types.NewCallTx(blockCache, user[2].PubKey, nil, nil, 100, 100, 100)
	tx.Sign(chainID, user[2])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}
}

func TestSendPermission(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Base.Set(ptypes.Send, true) // give the 0 account permission
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	// A single input, having the permission, should succeed
	tx := types.NewSendTx()
	if err := tx.AddInput(blockCache, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[1].Address, 5)
	tx.SignInput(chainID, 0, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Transaction failed", err)
	}

	// Two inputs, one with permission, one without, should fail
	tx = types.NewSendTx()
	if err := tx.AddInput(blockCache, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	if err := tx.AddInput(blockCache, user[1].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[2].Address, 10)
	tx.SignInput(chainID, 0, user[0])
	tx.SignInput(chainID, 1, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}
}

func TestCallPermission(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Base.Set(ptypes.Call, true) // give the 0 account permission
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//------------------------------
	// call to simple contract
	fmt.Println("##### SIMPLE CONTRACT")

	// create simple contract
	simpleContractAddr := NewContractAddress(user[0].Address, 100)
	simpleAcc := &account.Account{
		Address:     simpleContractAddr,
		Balance:     0,
		Code:        []byte{0x60},
		Sequence:    0,
		StorageRoot: Zero256.Bytes(),
		Permissions: ptypes.NewAccountPermissions(),
	}
	st.UpdateAccount(simpleAcc)

	// A single input, having the permission, should succeed
	tx, _ := types.NewCallTx(blockCache, user[0].PubKey, simpleContractAddr, nil, 100, 100, 100)
	tx.Sign(chainID, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Transaction failed", err)
	}

	//----------------------------------------------------------
	// call to contract that calls simple contract - without perm
	fmt.Println("##### CALL TO SIMPLE CONTRACT (FAIL)")

	// create contract that calls the simple contract
	contractCode := callContractCode(simpleContractAddr)
	caller1ContractAddr := NewContractAddress(user[0].Address, 101)
	caller1Acc := &account.Account{
		Address:     caller1ContractAddr,
		Balance:     0,
		Code:        contractCode,
		Sequence:    0,
		StorageRoot: Zero256.Bytes(),
		Permissions: ptypes.NewAccountPermissions(),
	}
	blockCache.UpdateAccount(caller1Acc)

	// A single input, having the permission, but the contract doesn't have permission
	tx, _ = types.NewCallTx(blockCache, user[0].PubKey, caller1ContractAddr, nil, 100, 10000, 100)
	tx.Sign(chainID, user[0])

	// we need to subscribe to the Receive event to detect the exception
	_, exception := execTxWaitEvent(t, blockCache, tx, types.EventStringAccReceive(caller1ContractAddr)) //
	if exception == "" {
		t.Fatal("Expected exception")
	}

	//----------------------------------------------------------
	// call to contract that calls simple contract - with perm
	fmt.Println("##### CALL TO SIMPLE CONTRACT (PASS)")

	// A single input, having the permission, and the contract has permission
	caller1Acc.Permissions.Base.Set(ptypes.Call, true)
	blockCache.UpdateAccount(caller1Acc)
	tx, _ = types.NewCallTx(blockCache, user[0].PubKey, caller1ContractAddr, nil, 100, 10000, 100)
	tx.Sign(chainID, user[0])

	// we need to subscribe to the Receive event to detect the exception
	_, exception = execTxWaitEvent(t, blockCache, tx, types.EventStringAccReceive(caller1ContractAddr)) //
	if exception != "" {
		t.Fatal("Unexpected exception:", exception)
	}

	//----------------------------------------------------------
	// call to contract that calls contract that calls simple contract - without perm
	// caller1Contract calls simpleContract. caller2Contract calls caller1Contract.
	// caller1Contract does not have call perms, but caller2Contract does.
	fmt.Println("##### CALL TO CONTRACT CALLING SIMPLE CONTRACT (FAIL)")

	contractCode2 := callContractCode(caller1ContractAddr)
	caller2ContractAddr := NewContractAddress(user[0].Address, 102)
	caller2Acc := &account.Account{
		Address:     caller2ContractAddr,
		Balance:     1000,
		Code:        contractCode2,
		Sequence:    0,
		StorageRoot: Zero256.Bytes(),
		Permissions: ptypes.NewAccountPermissions(),
	}
	caller1Acc.Permissions.Base.Set(ptypes.Call, false)
	caller2Acc.Permissions.Base.Set(ptypes.Call, true)
	blockCache.UpdateAccount(caller1Acc)
	blockCache.UpdateAccount(caller2Acc)

	tx, _ = types.NewCallTx(blockCache, user[0].PubKey, caller2ContractAddr, nil, 100, 10000, 100)
	tx.Sign(chainID, user[0])

	// we need to subscribe to the Receive event to detect the exception
	_, exception = execTxWaitEvent(t, blockCache, tx, types.EventStringAccReceive(caller1ContractAddr)) //
	if exception == "" {
		t.Fatal("Expected exception")
	}

	//----------------------------------------------------------
	// call to contract that calls contract that calls simple contract - without perm
	// caller1Contract calls simpleContract. caller2Contract calls caller1Contract.
	// both caller1 and caller2 have permission
	fmt.Println("##### CALL TO CONTRACT CALLING SIMPLE CONTRACT (PASS)")

	caller1Acc.Permissions.Base.Set(ptypes.Call, true)
	blockCache.UpdateAccount(caller1Acc)

	tx, _ = types.NewCallTx(blockCache, user[0].PubKey, caller2ContractAddr, nil, 100, 10000, 100)
	tx.Sign(chainID, user[0])

	// we need to subscribe to the Receive event to detect the exception
	_, exception = execTxWaitEvent(t, blockCache, tx, types.EventStringAccReceive(caller1ContractAddr)) //
	if exception != "" {
		t.Fatal("Unexpected exception", exception)
	}
}

func TestCreatePermission(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Base.Set(ptypes.CreateContract, true) // give the 0 account permission
	genDoc.Accounts[0].Permissions.Base.Set(ptypes.Call, true)           // give the 0 account permission
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//------------------------------
	// create a simple contract
	fmt.Println("##### CREATE SIMPLE CONTRACT")

	contractCode := []byte{0x60}
	createCode := wrapContractForCreate(contractCode)

	// A single input, having the permission, should succeed
	tx, _ := types.NewCallTx(blockCache, user[0].PubKey, nil, createCode, 100, 100, 100)
	tx.Sign(chainID, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Transaction failed", err)
	}
	// ensure the contract is there
	contractAddr := NewContractAddress(tx.Input.Address, uint64(tx.Input.Sequence))
	contractAcc := blockCache.GetAccount(contractAddr)
	if contractAcc == nil {
		t.Fatalf("failed to create contract %X", contractAddr)
	}
	if bytes.Compare(contractAcc.Code, contractCode) != 0 {
		t.Fatalf("contract does not have correct code. Got %X, expected %X", contractAcc.Code, contractCode)
	}

	//------------------------------
	// create contract that uses the CREATE op
	fmt.Println("##### CREATE FACTORY")

	contractCode = []byte{0x60}
	createCode = wrapContractForCreate(contractCode)
	factoryCode := createContractCode()
	createFactoryCode := wrapContractForCreate(factoryCode)

	// A single input, having the permission, should succeed
	tx, _ = types.NewCallTx(blockCache, user[0].PubKey, nil, createFactoryCode, 100, 100, 100)
	tx.Sign(chainID, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Transaction failed", err)
	}
	// ensure the contract is there
	contractAddr = NewContractAddress(tx.Input.Address, uint64(tx.Input.Sequence))
	contractAcc = blockCache.GetAccount(contractAddr)
	if contractAcc == nil {
		t.Fatalf("failed to create contract %X", contractAddr)
	}
	if bytes.Compare(contractAcc.Code, factoryCode) != 0 {
		t.Fatalf("contract does not have correct code. Got %X, expected %X", contractAcc.Code, factoryCode)
	}

	//------------------------------
	// call the contract (should FAIL)
	fmt.Println("###### CALL THE FACTORY (FAIL)")

	// A single input, having the permission, should succeed
	tx, _ = types.NewCallTx(blockCache, user[0].PubKey, contractAddr, createCode, 100, 100, 100)
	tx.Sign(chainID, user[0])
	// we need to subscribe to the Receive event to detect the exception
	_, exception := execTxWaitEvent(t, blockCache, tx, types.EventStringAccReceive(contractAddr)) //
	if exception == "" {
		t.Fatal("expected exception")
	}

	//------------------------------
	// call the contract (should PASS)
	fmt.Println("###### CALL THE FACTORY (PASS)")

	contractAcc.Permissions.Base.Set(ptypes.CreateContract, true)
	blockCache.UpdateAccount(contractAcc)

	// A single input, having the permission, should succeed
	tx, _ = types.NewCallTx(blockCache, user[0].PubKey, contractAddr, createCode, 100, 100, 100)
	tx.Sign(chainID, user[0])
	// we need to subscribe to the Receive event to detect the exception
	_, exception = execTxWaitEvent(t, blockCache, tx, types.EventStringAccReceive(contractAddr)) //
	if exception != "" {
		t.Fatal("unexpected exception", exception)
	}

	//--------------------------------
	fmt.Println("##### CALL to empty address")
	zeroAddr := LeftPadBytes([]byte{}, 20)
	code := callContractCode(zeroAddr)

	contractAddr = NewContractAddress(user[0].Address, 110)
	contractAcc = &account.Account{
		Address:     contractAddr,
		Balance:     1000,
		Code:        code,
		Sequence:    0,
		StorageRoot: Zero256.Bytes(),
		Permissions: ptypes.NewAccountPermissions(),
	}
	contractAcc.Permissions.Base.Set(ptypes.Call, true)
	contractAcc.Permissions.Base.Set(ptypes.CreateContract, true)
	blockCache.UpdateAccount(contractAcc)

	// this should call the 0 address but not create ...
	tx, _ = types.NewCallTx(blockCache, user[0].PubKey, contractAddr, createCode, 100, 10000, 100)
	tx.Sign(chainID, user[0])
	// we need to subscribe to the Receive event to detect the exception
	_, exception = execTxWaitEvent(t, blockCache, tx, types.EventStringAccReceive(zeroAddr)) //
	if exception != "" {
		t.Fatal("unexpected exception", exception)
	}
	zeroAcc := blockCache.GetAccount(zeroAddr)
	if len(zeroAcc.Code) != 0 {
		t.Fatal("the zero account was given code from a CALL!")
	}
}

func TestBondPermission(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)
	var bondAcc *account.Account

	//------------------------------
	// one bonder without permission should fail
	tx, _ := types.NewBondTx(user[1].PubKey)
	if err := tx.AddInput(blockCache, user[1].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[1].Address, 5)
	tx.SignInput(chainID, 0, user[1])
	tx.SignBond(chainID, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	//------------------------------
	// one bonder with permission should pass
	bondAcc = blockCache.GetAccount(user[1].Address)
	bondAcc.Permissions.Base.Set(ptypes.Bond, true)
	blockCache.UpdateAccount(bondAcc)
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Unexpected error", err)
	}

	// reset state (we can only bond with an account once ..)
	genDoc = newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	st = MakeGenesisState(stateDB, &genDoc)
	blockCache = NewBlockCache(st)
	bondAcc = blockCache.GetAccount(user[1].Address)
	bondAcc.Permissions.Base.Set(ptypes.Bond, true)
	blockCache.UpdateAccount(bondAcc)
	//------------------------------
	// one bonder with permission and an input without send should fail
	tx, _ = types.NewBondTx(user[1].PubKey)
	if err := tx.AddInput(blockCache, user[2].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[1].Address, 5)
	tx.SignInput(chainID, 0, user[2])
	tx.SignBond(chainID, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// reset state (we can only bond with an account once ..)
	genDoc = newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	st = MakeGenesisState(stateDB, &genDoc)
	blockCache = NewBlockCache(st)
	bondAcc = blockCache.GetAccount(user[1].Address)
	bondAcc.Permissions.Base.Set(ptypes.Bond, true)
	blockCache.UpdateAccount(bondAcc)
	//------------------------------
	// one bonder with permission and an input with send should pass
	sendAcc := blockCache.GetAccount(user[2].Address)
	sendAcc.Permissions.Base.Set(ptypes.Send, true)
	blockCache.UpdateAccount(sendAcc)
	tx, _ = types.NewBondTx(user[1].PubKey)
	if err := tx.AddInput(blockCache, user[2].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[1].Address, 5)
	tx.SignInput(chainID, 0, user[2])
	tx.SignBond(chainID, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Unexpected error", err)
	}

	// reset state (we can only bond with an account once ..)
	genDoc = newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	st = MakeGenesisState(stateDB, &genDoc)
	blockCache = NewBlockCache(st)
	bondAcc = blockCache.GetAccount(user[1].Address)
	bondAcc.Permissions.Base.Set(ptypes.Bond, true)
	blockCache.UpdateAccount(bondAcc)
	//------------------------------
	// one bonder with permission and an input with bond should pass
	sendAcc.Permissions.Base.Set(ptypes.Bond, true)
	blockCache.UpdateAccount(sendAcc)
	tx, _ = types.NewBondTx(user[1].PubKey)
	if err := tx.AddInput(blockCache, user[2].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[1].Address, 5)
	tx.SignInput(chainID, 0, user[2])
	tx.SignBond(chainID, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Unexpected error", err)
	}

	// reset state (we can only bond with an account once ..)
	genDoc = newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	st = MakeGenesisState(stateDB, &genDoc)
	blockCache = NewBlockCache(st)
	bondAcc = blockCache.GetAccount(user[1].Address)
	bondAcc.Permissions.Base.Set(ptypes.Bond, true)
	blockCache.UpdateAccount(bondAcc)
	//------------------------------
	// one bonder with permission and an input from that bonder and an input without send or bond should fail
	tx, _ = types.NewBondTx(user[1].PubKey)
	if err := tx.AddInput(blockCache, user[1].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	if err := tx.AddInput(blockCache, user[2].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[1].Address, 5)
	tx.SignInput(chainID, 0, user[1])
	tx.SignInput(chainID, 1, user[2])
	tx.SignBond(chainID, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	}
}

func TestCreateAccountPermission(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Base.Set(ptypes.Send, true)          // give the 0 account permission
	genDoc.Accounts[1].Permissions.Base.Set(ptypes.Send, true)          // give the 0 account permission
	genDoc.Accounts[0].Permissions.Base.Set(ptypes.CreateAccount, true) // give the 0 account permission
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//----------------------------------------------------------
	// SendTx to unknown account

	// A single input, having the permission, should succeed
	tx := types.NewSendTx()
	if err := tx.AddInput(blockCache, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[6].Address, 5)
	tx.SignInput(chainID, 0, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Transaction failed", err)
	}

	// Two inputs, both with send, one with create, one without, should fail
	tx = types.NewSendTx()
	if err := tx.AddInput(blockCache, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	if err := tx.AddInput(blockCache, user[1].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[7].Address, 10)
	tx.SignInput(chainID, 0, user[0])
	tx.SignInput(chainID, 1, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// Two inputs, both with send, one with create, one without, two ouputs (one known, one unknown) should fail
	tx = types.NewSendTx()
	if err := tx.AddInput(blockCache, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	if err := tx.AddInput(blockCache, user[1].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[7].Address, 4)
	tx.AddOutput(user[4].Address, 6)
	tx.SignInput(chainID, 0, user[0])
	tx.SignInput(chainID, 1, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// Two inputs, both with send, both with create, should pass
	acc := blockCache.GetAccount(user[1].Address)
	acc.Permissions.Base.Set(ptypes.CreateAccount, true)
	blockCache.UpdateAccount(acc)
	tx = types.NewSendTx()
	if err := tx.AddInput(blockCache, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	if err := tx.AddInput(blockCache, user[1].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[7].Address, 10)
	tx.SignInput(chainID, 0, user[0])
	tx.SignInput(chainID, 1, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Unexpected error", err)
	}

	// Two inputs, both with send, both with create, two outputs (one known, one unknown) should pass
	tx = types.NewSendTx()
	if err := tx.AddInput(blockCache, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	if err := tx.AddInput(blockCache, user[1].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	tx.AddOutput(user[7].Address, 7)
	tx.AddOutput(user[4].Address, 3)
	tx.SignInput(chainID, 0, user[0])
	tx.SignInput(chainID, 1, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Unexpected error", err)
	}

	//----------------------------------------------------------
	// CALL to unknown account

	acc = blockCache.GetAccount(user[0].Address)
	acc.Permissions.Base.Set(ptypes.Call, true)
	blockCache.UpdateAccount(acc)

	// call to contract that calls unknown account - without create_account perm
	// create contract that calls the simple contract
	contractCode := callContractCode(user[9].Address)
	caller1ContractAddr := NewContractAddress(user[4].Address, 101)
	caller1Acc := &account.Account{
		Address:     caller1ContractAddr,
		Balance:     0,
		Code:        contractCode,
		Sequence:    0,
		StorageRoot: Zero256.Bytes(),
		Permissions: ptypes.NewAccountPermissions(),
	}
	blockCache.UpdateAccount(caller1Acc)

	// A single input, having the permission, but the contract doesn't have permission
	txCall, _ := types.NewCallTx(blockCache, user[0].PubKey, caller1ContractAddr, nil, 100, 10000, 100)
	txCall.Sign(chainID, user[0])

	// we need to subscribe to the Receive event to detect the exception
	_, exception := execTxWaitEvent(t, blockCache, txCall, types.EventStringAccReceive(caller1ContractAddr)) //
	if exception == "" {
		t.Fatal("Expected exception")
	}

	// NOTE: for a contract to be able to CreateAccount, it must be able to call
	// NOTE: for a user to be able to CreateAccount, it must be able to send!
	caller1Acc.Permissions.Base.Set(ptypes.CreateAccount, true)
	caller1Acc.Permissions.Base.Set(ptypes.Call, true)
	blockCache.UpdateAccount(caller1Acc)
	// A single input, having the permission, but the contract doesn't have permission
	txCall, _ = types.NewCallTx(blockCache, user[0].PubKey, caller1ContractAddr, nil, 100, 10000, 100)
	txCall.Sign(chainID, user[0])

	// we need to subscribe to the Receive event to detect the exception
	_, exception = execTxWaitEvent(t, blockCache, txCall, types.EventStringAccReceive(caller1ContractAddr)) //
	if exception != "" {
		t.Fatal("Unexpected exception", exception)
	}

}

func TestSNativeCALL(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Base.Set(ptypes.Call, true) // give the 0 account permission
	genDoc.Accounts[3].Permissions.Base.Set(ptypes.Bond, true) // some arbitrary permission to play with
	genDoc.Accounts[3].Permissions.AddRole("bumble")
	genDoc.Accounts[3].Permissions.AddRole("bee")
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//----------------------------------------------------------
	// Test CALL to SNative contracts

	// make the main contract once
	doug := &account.Account{
		Address:     ptypes.DougAddress,
		Balance:     0,
		Code:        nil,
		Sequence:    0,
		StorageRoot: Zero256.Bytes(),
		Permissions: ptypes.NewAccountPermissions(),
	}
	doug.Permissions.Base.Set(ptypes.Call, true)
	blockCache.UpdateAccount(doug)

	fmt.Println("#### hasBasePerm")
	// hasBasePerm
	snativeAddress, data := snativePermTestInput("hasBasePerm", user[3], ptypes.Bond, false)
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		// return value should be true or false as a 32 byte array...
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### setBasePerm")
	// setBasePerm
	snativeAddress, data = snativePermTestInput("setBasePerm", user[3], ptypes.Bond, false)
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativePermTestInput("hasBasePerm", user[3], ptypes.Bond, false)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		// return value should be true or false as a 32 byte array...
		if !IsZeros(ret) {
			return fmt.Errorf("Expected 0. Got %X", ret)
		}
		return nil
	})
	snativeAddress, data = snativePermTestInput("setBasePerm", user[3], ptypes.CreateContract, true)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativePermTestInput("hasBasePerm", user[3], ptypes.CreateContract, false)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		// return value should be true or false as a 32 byte array...
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### unsetBasePerm")
	// unsetBasePerm
	snativeAddress, data = snativePermTestInput("unsetBasePerm", user[3], ptypes.CreateContract, false)
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativePermTestInput("hasBasePerm", user[3], ptypes.CreateContract, false)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret) {
			return fmt.Errorf("Expected 0. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### setGlobalPerm")
	// setGlobalPerm
	snativeAddress, data = snativePermTestInput("setGlobalPerm", user[3], ptypes.CreateContract, true)
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativePermTestInput("hasBasePerm", user[3], ptypes.CreateContract, false)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		// return value should be true or false as a 32 byte array...
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	// clearBasePerm
	// TODO

	fmt.Println("#### hasRole")
	// hasRole
	snativeAddress, data = snativeRoleTestInput("hasRole", user[3], "bumble")
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### addRole")
	// addRole
	snativeAddress, data = snativeRoleTestInput("hasRole", user[3], "chuck")
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret) {
			return fmt.Errorf("Expected 0. Got %X", ret)
		}
		return nil
	})
	snativeAddress, data = snativeRoleTestInput("addRole", user[3], "chuck")
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativeRoleTestInput("hasRole", user[3], "chuck")
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### rmRole")
	// rmRole
	snativeAddress, data = snativeRoleTestInput("rmRole", user[3], "chuck")
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativeRoleTestInput("hasRole", user[3], "chuck")
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret) {
			return fmt.Errorf("Expected 0. Got %X", ret)
		}
		return nil
	})
}

func TestSNativeCallTx(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Base.Set(ptypes.Call, true) // give the 0 account permission
	genDoc.Accounts[3].Permissions.Base.Set(ptypes.Bond, true) // some arbitrary permission to play with
	genDoc.Accounts[3].Permissions.AddRole("bumble")
	genDoc.Accounts[3].Permissions.AddRole("bee")
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//----------------------------------------------------------
	// Test CallTx to SNative contracts
	var doug *account.Account = nil

	fmt.Println("#### hasBasePerm")
	// hasBasePerm
	snativeAddress, data := snativePermTestInput("hasBasePerm", user[3], ptypes.Bond, false)
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		// return value should be true or false as a 32 byte array...
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### setBasePerm")
	// setBasePerm
	snativeAddress, data = snativePermTestInput("setBasePerm", user[3], ptypes.Bond, false)
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativePermTestInput("hasBasePerm", user[3], ptypes.Bond, false)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		// return value should be true or false as a 32 byte array...
		if !IsZeros(ret) {
			return fmt.Errorf("Expected 0. Got %X", ret)
		}
		return nil
	})
	snativeAddress, data = snativePermTestInput("setBasePerm", user[3], ptypes.CreateContract, true)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativePermTestInput("hasBasePerm", user[3], ptypes.CreateContract, false)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		// return value should be true or false as a 32 byte array...
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### unsetBasePerm")
	// unsetBasePerm
	snativeAddress, data = snativePermTestInput("unsetBasePerm", user[3], ptypes.CreateContract, false)
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativePermTestInput("hasBasePerm", user[3], ptypes.CreateContract, false)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret) {
			return fmt.Errorf("Expected 0. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### setGlobalPerm")
	// setGlobalPerm
	snativeAddress, data = snativePermTestInput("setGlobalPerm", user[3], ptypes.CreateContract, true)
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativePermTestInput("hasBasePerm", user[3], ptypes.CreateContract, false)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		// return value should be true or false as a 32 byte array...
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	// clearBasePerm
	// TODO

	fmt.Println("#### hasRole")
	// hasRole
	snativeAddress, data = snativeRoleTestInput("hasRole", user[3], "bumble")
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### addRole")
	// addRole
	snativeAddress, data = snativeRoleTestInput("hasRole", user[3], "chuck")
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret) {
			return fmt.Errorf("Expected 0. Got %X", ret)
		}
		return nil
	})
	snativeAddress, data = snativeRoleTestInput("addRole", user[3], "chuck")
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativeRoleTestInput("hasRole", user[3], "chuck")
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret[:31]) || ret[31] != byte(1) {
			return fmt.Errorf("Expected 1. Got %X", ret)
		}
		return nil
	})

	fmt.Println("#### rmRole")
	// rmRole
	snativeAddress, data = snativeRoleTestInput("rmRole", user[3], "chuck")
	testSNativeCALLExpectFail(t, blockCache, doug, snativeAddress, data)
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error { return nil })
	snativeAddress, data = snativeRoleTestInput("hasRole", user[3], "chuck")
	testSNativeCALLExpectPass(t, blockCache, doug, snativeAddress, data, func(ret []byte) error {
		if !IsZeros(ret) {
			return fmt.Errorf("Expected 0. Got %X", ret)
		}
		return nil
	})
}

//-------------------------------------------------------------------------------------
// helpers

// run ExecTx and wait for the Receive event on given addr
// returns the msg data and an error/exception
func execTxWaitEvent(t *testing.T, blockCache *BlockCache, tx types.Tx, eventid string) (interface{}, string) {
	evsw := new(events.EventSwitch)
	evsw.Start()
	ch := make(chan interface{})
	evsw.AddListenerForEvent("test", eventid, func(msg interface{}) {
		ch <- msg
	})
	evc := events.NewEventCache(evsw)
	go func() {
		if err := ExecTx(blockCache, tx, true, evc); err != nil {
			ch <- err.Error()
		}
		evc.Flush()
	}()
	msg := <-ch
	switch ev := msg.(type) {
	case types.EventMsgCallTx:
		return ev, ev.Exception
	case types.EventMsgCall:
		return ev, ev.Exception
	case string:
		return nil, ev
	default:
		return ev, ""
	}
}

// give a contract perms for an snative, call it, it calls the snative, ensure the check funciton (f) succeeds
func testSNativeCALLExpectPass(t *testing.T, blockCache *BlockCache, doug *account.Account, snativeAddress, data []byte, f func([]byte) error) {
	perm := vm.RegisteredSNativePermissions[LeftPadWord256(snativeAddress)]
	var addr []byte
	if doug != nil {
		contractCode := callContractCode(snativeAddress)
		doug.Code = contractCode
		doug.Permissions.Base.Set(perm, true)
		blockCache.UpdateAccount(doug)
		addr = doug.Address
	} else {
		acc := blockCache.GetAccount(user[0].Address)
		acc.Permissions.Base.Set(perm, true)
		blockCache.UpdateAccount(acc)
		addr = snativeAddress
	}
	tx, _ := types.NewCallTx(blockCache, user[0].PubKey, addr, data, 100, 10000, 100)
	tx.Sign(chainID, user[0])
	ev, exception := execTxWaitEvent(t, blockCache, tx, types.EventStringAccReceive(snativeAddress)) //
	if exception != "" {
		t.Fatal("Unexpected exception", exception)
	}
	evv := ev.(types.EventMsgCall)
	ret := evv.Return
	if err := f(ret); err != nil {
		t.Fatal(err)
	}
}

// assumes the contract has not been given the permission. calls the it, it calls the snative, expects to fail
func testSNativeCALLExpectFail(t *testing.T, blockCache *BlockCache, doug *account.Account, snativeAddress, data []byte) {
	var addr []byte
	if doug != nil {
		contractCode := callContractCode(snativeAddress)
		doug.Code = contractCode
		blockCache.UpdateAccount(doug)
		addr = doug.Address
	} else {
		addr = snativeAddress
	}
	tx, _ := types.NewCallTx(blockCache, user[0].PubKey, addr, data, 100, 10000, 100)
	tx.Sign(chainID, user[0])
	fmt.Println("subscribing to", types.EventStringAccReceive(snativeAddress))
	_, exception := execTxWaitEvent(t, blockCache, tx, types.EventStringAccReceive(snativeAddress))
	if exception == "" {
		t.Fatal("Expected exception")
	}
}

func boolToWord256(v bool) Word256 {
	var vint byte
	if v {
		vint = 0x1
	} else {
		vint = 0x0
	}
	return LeftPadWord256([]byte{vint})
}

func snativePermTestInput(name string, user *account.PrivAccount, perm ptypes.PermFlag, val bool) (addr []byte, data []byte) {
	addr = LeftPadWord256([]byte(name)).Postfix(20)
	switch name {
	case "hasBasePerm", "unsetBasePerm":
		data = LeftPadBytes(user.Address, 32)
		data = append(data, Uint64ToWord256(uint64(perm)).Bytes()...)
	case "setBasePerm":
		data = LeftPadBytes(user.Address, 32)
		data = append(data, Uint64ToWord256(uint64(perm)).Bytes()...)
		data = append(data, boolToWord256(val).Bytes()...)
	case "setGlobalPerm":
		data = Uint64ToWord256(uint64(perm)).Bytes()
		data = append(data, boolToWord256(val).Bytes()...)
	case "clearBasePerm":
	}
	return
}

func snativeRoleTestInput(name string, user *account.PrivAccount, role string) (addr []byte, data []byte) {
	addr = LeftPadWord256([]byte(name)).Postfix(20)
	data = LeftPadBytes(user.Address, 32)
	data = append(data, LeftPadBytes([]byte(role), 32)...)
	return
}

// convenience function for contract that calls a given address
func callContractCode(contractAddr []byte) []byte {
	// calldatacopy into mem and use as input to call
	memOff, inputOff := byte(0x0), byte(0x0)
	contractCode := []byte{0x36, 0x60, inputOff, 0x60, memOff, 0x37}

	gas1, gas2 := byte(0x1), byte(0x1)
	value := byte(0x1)
	inOff := byte(0x0)
	retOff, retSize := byte(0x0), byte(0x20)
	// this is the code we want to run (call a contract and return)
	contractCode = append(contractCode, []byte{0x60, retSize, 0x60, retOff, 0x36, 0x60, inOff, 0x60, value, 0x73}...)
	contractCode = append(contractCode, contractAddr...)
	contractCode = append(contractCode, []byte{0x61, gas1, gas2, 0xf1, 0x60, 0x20, 0x60, 0x0, 0xf3}...)
	return contractCode
}

// convenience function for contract that is a factory for the code that comes as call data
func createContractCode() []byte {
	// TODO: gas ...

	// calldatacopy the calldatasize
	memOff, inputOff := byte(0x0), byte(0x0)
	contractCode := []byte{0x60, memOff, 0x60, inputOff, 0x36, 0x37}

	// create
	value := byte(0x1)
	contractCode = append(contractCode, []byte{0x60, value, 0x36, 0x60, memOff, 0xf0}...)
	return contractCode
}

// wrap a contract in create code
func wrapContractForCreate(contractCode []byte) []byte {
	// the is the code we need to return the contractCode when the contract is initialized
	lenCode := len(contractCode)
	// push code to the stack
	code := append([]byte{0x7f}, RightPadWord256(contractCode).Bytes()...)
	// store it in memory
	code = append(code, []byte{0x60, 0x0, 0x52}...)
	// return whats in memory
	code = append(code, []byte{0x60, byte(lenCode), 0x60, 0x0, 0xf3}...)
	// return init code, contract code, expected return
	return code
}
