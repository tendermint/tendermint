package main

import (
	"fmt"
	btypes "github.com/tendermint/tendermint/cmd/barak/types"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/rpc"
	// ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func main() {
	response := []btypes.ResponseListProcesses{}
	response2, err := rpc.Call("http://127.0.0.1:8082", "list_processes", Arr(), &response)
	fmt.Printf("%v\n", response)
	fmt.Printf("%v (error: %v)\n", response2, err)
}
