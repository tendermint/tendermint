package main

import (
	"fmt"
	"os"

	amino "github.com/tendermint/go-amino"
	cryptoAmino "github.com/pakula/prism/crypto/encoding/amino"
)

func main() {
	cdc := amino.NewCodec()
	cryptoAmino.RegisterAmino(cdc)
	cdc.PrintTypes(os.Stdout)
	fmt.Println("")
}
