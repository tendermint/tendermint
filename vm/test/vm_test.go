package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	. "github.com/tendermint/tendermint/vm"
	"strings"
	"testing"
	"time"
)

func newAppState() *FakeAppState {
	return &FakeAppState{
		accounts: make(map[string]*Account),
		storage:  make(map[string]Word),
		logs:     nil,
	}
}

func newParams() Params {
	return Params{
		BlockHeight: 0,
		BlockHash:   Zero,
		BlockTime:   0,
		GasLimit:    0,
	}
}

func makeBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func TestVM(t *testing.T) {
	ourVm := NewVM(newAppState(), newParams(), Zero)

	// Create accounts
	account1 := &Account{
		Address: Uint64ToWord(100),
	}
	account2 := &Account{
		Address: Uint64ToWord(101),
	}

	var gas uint64 = 1000
	N := []byte{0xff, 0xff}
	// Loop N times
	code := []byte{0x60, 0x00, 0x60, 0x20, 0x52, 0x5B, byte(0x60 + len(N) - 1)}
	for i := 0; i < len(N); i++ {
		code = append(code, N[i])
	}
	code = append(code, []byte{0x60, 0x20, 0x51, 0x12, 0x15, 0x60, byte(0x1b + len(N)), 0x57, 0x60, 0x01, 0x60, 0x20, 0x51, 0x01, 0x60, 0x20, 0x52, 0x60, 0x05, 0x56, 0x5B}...)
	start := time.Now()
	output, err := ourVm.Call(account1, account2, code, []byte{}, 0, &gas)
	fmt.Printf("Output: %v Error: %v\n", output, err)
	fmt.Println("Call took:", time.Since(start))
}

func TestSubcurrency(t *testing.T) {
	st := newAppState()
	// Create accounts
	account1 := &Account{
		Address: BytesToWord(makeBytes(20)),
	}
	account2 := &Account{
		Address: BytesToWord(makeBytes(20)),
	}
	st.accounts[account1.Address.String()] = account1
	st.accounts[account2.Address.String()] = account2

	ourVm := NewVM(st, newParams(), Zero)

	var gas uint64 = 1000
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

}

/*
	// infinite loop
	code := []byte{0x5B, 0x60, 0x00, 0x56}
	// mstore
	code := []byte{0x60, 0x00, 0x60, 0x20}
	// mstore, mload
	code := []byte{0x60, 0x01, 0x60, 0x20, 0x52, 0x60, 0x20, 0x51}
*/
