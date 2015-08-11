package vm

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
	ptypes "github.com/tendermint/tendermint/permission/types"
	"github.com/tendermint/tendermint/types"
	. "github.com/tendermint/tendermint/vm"
)

func newAppState() *FakeAppState {
	fas := &FakeAppState{
		accounts: make(map[string]*Account),
		storage:  make(map[string]Word256),
	}
	// For default permissions
	fas.accounts[ptypes.GlobalPermissionsAddress256.String()] = &Account{
		Permissions: ptypes.DefaultAccountPermissions,
	}
	return fas
}

func newParams() Params {
	return Params{
		BlockHeight: 0,
		BlockHash:   Zero256,
		BlockTime:   0,
		GasLimit:    0,
	}
}

func makeBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

// Runs a basic loop
func TestVM(t *testing.T) {
	ourVm := NewVM(newAppState(), newParams(), Zero256, nil)

	// Create accounts
	account1 := &Account{
		Address: Int64ToWord256(100),
	}
	account2 := &Account{
		Address: Int64ToWord256(101),
	}

	var gas int64 = 100000
	N := []byte{0x0f, 0x0f}
	// Loop N times
	code := []byte{0x60, 0x00, 0x60, 0x20, 0x52, 0x5B, byte(0x60 + len(N) - 1)}
	code = append(code, N...)
	code = append(code, []byte{0x60, 0x20, 0x51, 0x12, 0x15, 0x60, byte(0x1b + len(N)), 0x57, 0x60, 0x01, 0x60, 0x20, 0x51, 0x01, 0x60, 0x20, 0x52, 0x60, 0x05, 0x56, 0x5B}...)
	start := time.Now()
	output, err := ourVm.Call(account1, account2, code, []byte{}, 0, &gas)
	fmt.Printf("Output: %v Error: %v\n", output, err)
	fmt.Println("Call took:", time.Since(start))
	if err != nil {
		t.Fatal(err)
	}
}

// Tests the code for a subcurrency contract compiled by serpent
func TestSubcurrency(t *testing.T) {
	st := newAppState()
	// Create accounts
	account1 := &Account{
		Address: LeftPadWord256(makeBytes(20)),
	}
	account2 := &Account{
		Address: LeftPadWord256(makeBytes(20)),
	}
	st.accounts[account1.Address.String()] = account1
	st.accounts[account2.Address.String()] = account2

	ourVm := NewVM(st, newParams(), Zero256, nil)

	var gas int64 = 1000
	code_parts := []string{"620f42403355",
		"7c0100000000000000000000000000000000000000000000000000000000",
		"600035046315cf268481141561004657",
		"6004356040526040515460605260206060f35b63693200ce81141561008757",
		"60043560805260243560a052335460c0523360e05260a05160c05112151561008657",
		"60a05160c0510360e0515560a0516080515401608051555b5b505b6000f3"}
	code, _ := hex.DecodeString(strings.Join(code_parts, ""))
	fmt.Printf("Code: %x\n", code)
	data, _ := hex.DecodeString("693200CE0000000000000000000000004B4363CDE27C2EB05E66357DB05BC5C88F850C1A0000000000000000000000000000000000000000000000000000000000000005")
	output, err := ourVm.Call(account1, account2, code, data, 0, &gas)
	fmt.Printf("Output: %v Error: %v\n", output, err)
	if err != nil {
		t.Fatal(err)
	}
}

// Test sending tokens from a contract to another account
func TestSendCall(t *testing.T) {
	fakeAppState := newAppState()
	ourVm := NewVM(fakeAppState, newParams(), Zero256, nil)

	// Create accounts
	account1 := &Account{
		Address: Int64ToWord256(100),
	}
	account2 := &Account{
		Address: Int64ToWord256(101),
	}
	account3 := &Account{
		Address: Int64ToWord256(102),
	}

	// account1 will call account2 which will trigger CALL opcode to account3
	addr := account3.Address.Postfix(20)
	contractCode := callContractCode(addr)

	//----------------------------------------------
	// account2 has insufficient balance, should fail
	fmt.Println("Should fail with insufficient balance")

	exception := runVMWaitEvents(t, ourVm, account1, account2, addr, contractCode, 1000)
	if exception == "" {
		t.Fatal("Expected exception")
	}

	//----------------------------------------------
	// give account2 sufficient balance, should pass

	account2.Balance = 100000
	exception = runVMWaitEvents(t, ourVm, account1, account2, addr, contractCode, 1000)
	if exception != "" {
		t.Fatal("Unexpected exception", exception)
	}

	//----------------------------------------------
	// insufficient gas, should fail
	fmt.Println("Should fail with insufficient gas")

	account2.Balance = 100000
	exception = runVMWaitEvents(t, ourVm, account1, account2, addr, contractCode, 100)
	if exception == "" {
		t.Fatal("Expected exception")
	}
}

// subscribes to an AccCall, runs the vm, returns the exception
func runVMWaitEvents(t *testing.T, ourVm *VM, caller, callee *Account, subscribeAddr, contractCode []byte, gas int64) string {
	// we need to catch the event from the CALL to check for exceptions
	evsw := events.NewEventSwitch()
	evsw.Start()
	ch := make(chan interface{})
	fmt.Printf("subscribe to %x\n", subscribeAddr)
	evsw.AddListenerForEvent("test", types.EventStringAccCall(subscribeAddr), func(msg types.EventData) {
		ch <- msg
	})
	evc := events.NewEventCache(evsw)
	ourVm.SetFireable(evc)
	go func() {
		start := time.Now()
		output, err := ourVm.Call(caller, callee, contractCode, []byte{}, 0, &gas)
		fmt.Printf("Output: %v Error: %v\n", output, err)
		fmt.Println("Call took:", time.Since(start))
		if err != nil {
			ch <- err.Error()
		}
		evc.Flush()
	}()
	msg := <-ch
	switch ev := msg.(type) {
	case types.EventDataTx:
		return ev.Exception
	case types.EventDataCall:
		return ev.Exception
	case string:
		return ev
	}
	return ""
}

// this is code to call another contract (hardcoded as addr)
func callContractCode(addr []byte) []byte {
	gas1, gas2 := byte(0x1), byte(0x1)
	value := byte(0x69)
	inOff, inSize := byte(0x0), byte(0x0) // no call data
	retOff, retSize := byte(0x0), byte(0x20)
	// this is the code we want to run (send funds to an account and return)
	contractCode := []byte{0x60, retSize, 0x60, retOff, 0x60, inSize, 0x60, inOff, 0x60, value, 0x73}
	contractCode = append(contractCode, addr...)
	contractCode = append(contractCode, []byte{0x61, gas1, gas2, 0xf1, 0x60, 0x20, 0x60, 0x0, 0xf3}...)
	return contractCode
}

/*
	// infinite loop
	code := []byte{0x5B, 0x60, 0x00, 0x56}
	// mstore
	code := []byte{0x60, 0x00, 0x60, 0x20}
	// mstore, mload
	code := []byte{0x60, 0x01, 0x60, 0x20, 0x52, 0x60, 0x20, 0x51}
*/
