package state

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/types"
)

/*
To Test:

- SendTx:
x	- 1 input, no perm, call perm, create perm
x	- 1 input, perm
x	- 2 inputs, one with perm one without

- CallTx
x	- 1 input, no perm, send perm, create perm
x	- 1 input, perm
x	- contract runs call but doesn't have call perm
x	- contract runs call and has call perm
x	- contract runs call (with perm), runs contract that runs call (without perm)
x	- contract runs call (with perm), runs contract that runs call (with perm)

- CallTx (Create)
x	- 1 input, no perm, send perm, call perm
	- 1 input, perm
	- contract runs create but doesn't have create perm
	- contract runs create but has perm
	- contract runs call with empty address (has call perm, not create perm)
	- contract runs call with empty address (has call perm, and create perm)

- BondTx
	- 1 input, no perm
	- 1 input, perm
	- 2 inputs, one with perm one without

- Gendoug: get/set/add/rm
*/

// keys
var (
	user = makeUsers(5)
)

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
	PermsAllFalse = account.Permissions{
		Send:   false,
		Call:   false,
		Create: false,
		Bond:   false,
	}
)

func newBaseGenDoc(globalPerm, accountPerm account.Permissions) GenesisDoc {
	genAccounts := []GenesisAccount{}
	for _, u := range user[:5] {
		accPerm := accountPerm
		genAccounts = append(genAccounts, GenesisAccount{
			Address:     u.Address,
			Amount:      1000000,
			Permissions: &accPerm,
		})
	}

	return GenesisDoc{
		GenesisTime: time.Now(),
		Params: &GenesisParams{
			GlobalPermissions: &globalPerm,
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
	genDoc.Accounts[1].Permissions.Send = true
	genDoc.Accounts[2].Permissions.Call = true
	genDoc.Accounts[3].Permissions.Create = true
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//-------------------
	// send txs

	// simple send tx should fail
	tx := NewSendTx()
	if err := SendTxAddInput(blockCache, tx, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	SendTxAddOutput(tx, user[1].Address, 5)
	SignSendTx(tx, 0, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple send tx with call perm should fail
	tx = NewSendTx()
	if err := SendTxAddInput(blockCache, tx, user[2].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	SendTxAddOutput(tx, user[4].Address, 5)
	SignSendTx(tx, 0, user[2])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple send tx with create perm should fail
	tx = NewSendTx()
	if err := SendTxAddInput(blockCache, tx, user[3].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	SendTxAddOutput(tx, user[4].Address, 5)
	SignSendTx(tx, 0, user[3])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}
}

func TestCallFails(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[1].Permissions.Send = true
	genDoc.Accounts[2].Permissions.Call = true
	genDoc.Accounts[3].Permissions.Create = true
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//-------------------
	// call txs

	// simple call tx should fail
	tx, _ := NewCallTx(blockCache, user[0].PubKey, user[4].Address, nil, 100, 100, 100)
	SignCallTx(tx, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple call tx with send permission should fail
	tx, _ = NewCallTx(blockCache, user[1].PubKey, user[4].Address, nil, 100, 100, 100)
	SignCallTx(tx, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple call tx with create permission should fail
	tx, _ = NewCallTx(blockCache, user[3].PubKey, user[4].Address, nil, 100, 100, 100)
	SignCallTx(tx, user[3])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	//-------------------
	// create txs

	// simple call create tx should fail
	tx, _ = NewCallTx(blockCache, user[0].PubKey, nil, nil, 100, 100, 100)
	SignCallTx(tx, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple call create tx with send perm should fail
	tx, _ = NewCallTx(blockCache, user[1].PubKey, nil, nil, 100, 100, 100)
	SignCallTx(tx, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}

	// simple call create tx with call perm should fail
	tx, _ = NewCallTx(blockCache, user[2].PubKey, nil, nil, 100, 100, 100)
	SignCallTx(tx, user[2])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}
}

func TestSendPermission(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Send = true // give the 0 account permission
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	// A single input, having the permission, should succeed
	tx := NewSendTx()
	if err := SendTxAddInput(blockCache, tx, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	SendTxAddOutput(tx, user[1].Address, 5)
	SignSendTx(tx, 0, user[0])
	if err := ExecTx(blockCache, tx, true, nil); err != nil {
		t.Fatal("Transaction failed", err)
	}

	// Two inputs, one with permission, one without, should fail
	tx = NewSendTx()
	if err := SendTxAddInput(blockCache, tx, user[0].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	if err := SendTxAddInput(blockCache, tx, user[1].PubKey, 5); err != nil {
		t.Fatal(err)
	}
	SendTxAddOutput(tx, user[2].Address, 10)
	SignSendTx(tx, 0, user[0])
	SignSendTx(tx, 1, user[1])
	if err := ExecTx(blockCache, tx, true, nil); err == nil {
		t.Fatal("Expected error")
	} else {
		fmt.Println(err)
	}
}

// convenience function for contract that calls a given address
func callContractCode(contractAddr []byte) []byte {
	gas1, gas2 := byte(0x1), byte(0x1)
	value := byte(0x1)
	inOff, inSize := byte(0x0), byte(0x0) // no call data
	retOff, retSize := byte(0x0), byte(0x20)
	// this is the code we want to run (call a contract and return)
	contractCode := []byte{0x60, retSize, 0x60, retOff, 0x60, inSize, 0x60, inOff, 0x60, value, 0x73}
	contractCode = append(contractCode, contractAddr...)
	contractCode = append(contractCode, []byte{0x61, gas1, gas2, 0xf1, 0x60, 0x20, 0x60, 0x0, 0xf3}...)
	return contractCode
}

func TestCallPermission(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Call = true // give the 0 account permission
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
	}
	st.UpdateAccount(simpleAcc)

	// A single input, having the permission, should succeed
	tx, _ := NewCallTx(blockCache, user[0].PubKey, simpleContractAddr, nil, 100, 100, 100)
	SignCallTx(tx, user[0])
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
	}
	blockCache.UpdateAccount(caller1Acc)

	// A single input, having the permission, but the contract doesn't have permission
	tx, _ = NewCallTx(blockCache, user[0].PubKey, caller1ContractAddr, nil, 100, 10000, 100)
	SignCallTx(tx, user[0])

	// we need to subscribe to the Receive event to detect the exception
	exception := execTxWaitEvent(t, blockCache, tx, caller1ContractAddr) //
	if exception == "" {
		t.Fatal("Expected exception")
	}

	//----------------------------------------------------------
	// call to contract that calls simple contract - with perm
	fmt.Println("##### CALL TO SIMPLE CONTRACT (PASS)")

	// A single input, having the permission, and the contract has permission
	caller1Acc.Permissions.Call = true
	blockCache.UpdateAccount(caller1Acc)
	tx, _ = NewCallTx(blockCache, user[0].PubKey, caller1ContractAddr, nil, 100, 10000, 100)
	SignCallTx(tx, user[0])

	// we need to subscribe to the Receive event to detect the exception
	exception = execTxWaitEvent(t, blockCache, tx, caller1ContractAddr) //
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
	}
	caller1Acc.Permissions.Call = false
	caller2Acc.Permissions.Call = true
	blockCache.UpdateAccount(caller1Acc)
	blockCache.UpdateAccount(caller2Acc)

	tx, _ = NewCallTx(blockCache, user[0].PubKey, caller2ContractAddr, nil, 100, 10000, 100)
	SignCallTx(tx, user[0])

	// we need to subscribe to the Receive event to detect the exception
	exception = execTxWaitEvent(t, blockCache, tx, caller1ContractAddr) //
	if exception == "" {
		t.Fatal("Expected exception")
	}

	//----------------------------------------------------------
	// call to contract that calls contract that calls simple contract - without perm
	// caller1Contract calls simpleContract. caller2Contract calls caller1Contract.
	// both caller1 and caller2 have permission
	fmt.Println("##### CALL TO CONTRACT CALLING SIMPLE CONTRACT (PASS)")

	caller1Acc.Permissions.Call = true
	blockCache.UpdateAccount(caller1Acc)

	tx, _ = NewCallTx(blockCache, user[0].PubKey, caller2ContractAddr, nil, 100, 10000, 100)
	SignCallTx(tx, user[0])

	// we need to subscribe to the Receive event to detect the exception
	exception = execTxWaitEvent(t, blockCache, tx, caller1ContractAddr) //
	if exception != "" {
		t.Fatal("Unexpected exception", exception)
	}
}

// run ExecTx and wait for the Receive event on given addr
// returns error/exception
func execTxWaitEvent(t *testing.T, blockCache *BlockCache, tx types.Tx, addr []byte) string {
	evsw := new(events.EventSwitch)
	evsw.Start()
	ch := make(chan interface{})
	evsw.AddListenerForEvent("test", types.EventStringAccReceive(addr), func(msg interface{}) {
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
		return ev.Exception
	case types.EventMsgCall:
		return ev.Exception
	case string:
		return ev
	}
	return ""

}

/*
- CallTx (Create)
x	- 1 input, no perm, send perm, call perm
	- CallTx to create new contract, perm
	- contract runs create but doesn't have create perm
	- contract runs create but has perm
	- contract runs call with empty address (has call perm, not create perm)
	- contract runs call with empty address (has call perm, and create perm)
	- contract runs call (with perm), runs contract that runs create (without perm)
	- contract runs call (with perm), runs contract that runs create (with perm)
*/

// TODO: this is just a copy of CALL...
func TestCreatePermission(t *testing.T) {
	stateDB := dbm.GetDB("state")
	genDoc := newBaseGenDoc(PermsAllFalse, PermsAllFalse)
	genDoc.Accounts[0].Permissions.Call = true // give the 0 account permission
	st := MakeGenesisState(stateDB, &genDoc)
	blockCache := NewBlockCache(st)

	//------------------------------
	// create simple contract
	fmt.Println("##### SIMPLE CONTRACT")

	// create simple contract
	simpleContractAddr := NewContractAddress(user[0].Address, 100)
	simpleAcc := &account.Account{
		Address:     simpleContractAddr,
		Balance:     0,
		Code:        []byte{0x60},
		Sequence:    0,
		StorageRoot: Zero256.Bytes(),
	}
	st.UpdateAccount(simpleAcc)

	// A single input, having the permission, should succeed
	tx, _ := NewCallTx(blockCache, user[0].PubKey, simpleContractAddr, nil, 100, 100, 100)
	SignCallTx(tx, user[0])
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
	}
	blockCache.UpdateAccount(caller1Acc)

	// A single input, having the permission, but the contract doesn't have permission
	tx, _ = NewCallTx(blockCache, user[0].PubKey, caller1ContractAddr, nil, 100, 10000, 100)
	SignCallTx(tx, user[0])

	// we need to subscribe to the Receive event to detect the exception
	exception := execTxWaitEvent(t, blockCache, tx, caller1ContractAddr) //
	if exception == "" {
		t.Fatal("Expected exception")
	}

	//----------------------------------------------------------
	// call to contract that calls simple contract - with perm
	fmt.Println("##### CALL TO SIMPLE CONTRACT (PASS)")

	// A single input, having the permission, and the contract has permission
	caller1Acc.Permissions.Call = true
	blockCache.UpdateAccount(caller1Acc)
	tx, _ = NewCallTx(blockCache, user[0].PubKey, caller1ContractAddr, nil, 100, 10000, 100)
	SignCallTx(tx, user[0])

	// we need to subscribe to the Receive event to detect the exception
	exception = execTxWaitEvent(t, blockCache, tx, caller1ContractAddr) //
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
	}
	caller1Acc.Permissions.Call = false
	caller2Acc.Permissions.Call = true
	blockCache.UpdateAccount(caller1Acc)
	blockCache.UpdateAccount(caller2Acc)

	tx, _ = NewCallTx(blockCache, user[0].PubKey, caller2ContractAddr, nil, 100, 10000, 100)
	SignCallTx(tx, user[0])

	// we need to subscribe to the Receive event to detect the exception
	exception = execTxWaitEvent(t, blockCache, tx, caller1ContractAddr) //
	if exception == "" {
		t.Fatal("Expected exception")
	}

	//----------------------------------------------------------
	// call to contract that calls contract that calls simple contract - without perm
	// caller1Contract calls simpleContract. caller2Contract calls caller1Contract.
	// both caller1 and caller2 have permission
	fmt.Println("##### CALL TO CONTRACT CALLING SIMPLE CONTRACT (PASS)")

	caller1Acc.Permissions.Call = true
	blockCache.UpdateAccount(caller1Acc)

	tx, _ = NewCallTx(blockCache, user[0].PubKey, caller2ContractAddr, nil, 100, 10000, 100)
	SignCallTx(tx, user[0])

	// we need to subscribe to the Receive event to detect the exception
	exception = execTxWaitEvent(t, blockCache, tx, caller1ContractAddr) //
	if exception != "" {
		t.Fatal("Unexpected exception", exception)
	}
}
