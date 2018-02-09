package main

import (
	"fmt"
	"os"

	priv_val "github.com/tendermint/tendermint/types/priv_validator"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("USAGE: priv_val_converter <path to priv_validator.json>")
		os.Exit(1)
	}
	file := os.Args[1]
	_, err := priv_val.UpgradePrivValidator(file)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
