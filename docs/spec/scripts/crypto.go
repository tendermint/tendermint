package main

import (
	"fmt"
	"os"

	amino "github.com/tendermint/go-amino"
	cryptoamino "github.com/tendermint/tendermint/crypto/encoding/amino"
)

func main() {
	cdc := amino.NewCodec()
	cryptoamino.RegisterAmino(cdc)
	cdc.PrintTypes(os.Stdout)
	fmt.Println("")
}
