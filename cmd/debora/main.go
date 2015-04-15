package main

import (
	"fmt"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
)

func main() {
	var remote string = "http://127.0.0.1:8082"
	var err error
	var privKey acm.PrivKey
	binary.ReadJSON(&privKey, []byte(`
		[1,"PRIVKEYBYTES"]
	`), &err)
	response, err := ListProcesses(privKey, remote)
	fmt.Printf("%v (error: %v)\n", response, err)
}
