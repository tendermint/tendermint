package main

import (
	"fmt"
	. "github.com/tendermint/tendermint/vm"
	"testing"
	"time"
)

func newVM() *VM {
	appState := &FakeAppState{
		accounts: make(map[string]*Account),
		storage:  make(map[string]Word),
		logs:     nil,
	}
	params := Params{
		BlockHeight: 0,
		BlockHash:   Zero,
		BlockTime:   0,
		GasLimit:    0,
	}
	return NewVM(appState, params, Zero)
}

func TestVM(t *testing.T) {
	dbg
	ourVm := newVM()

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

/*
	// infinite loop
	code := []byte{0x5B, 0x60, 0x00, 0x56}
	// mstore
	code := []byte{0x60, 0x00, 0x60, 0x20}
	// mstore, mload
	code := []byte{0x60, 0x01, 0x60, 0x20, 0x52, 0x60, 0x20, 0x51}
*/
