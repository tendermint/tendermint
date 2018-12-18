package main

import (
	"fmt"
	"os"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
)

var (
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
)

func main() {
	args := os.Args[1:]
	if len(args) != 3 {
		fmt.Println("Expected three args: <old path> <new key path> <new state path>")
		fmt.Println("Eg. ~/.tendermint/config/priv_validator.json ~/.tendermint/config/priv_validator_key.json ~/.tendermint/data/priv_validator_state.json")
		os.Exit(1)
	}
	err := loadAndUpgrade(args[0], args[1], args[2])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func loadAndUpgrade(oldPVPath, newPVKeyPath, newPVStatePath string) error {
	oldPV, err := privval.LoadOldFilePV(oldPVPath)
	if err != nil {
		return fmt.Errorf("Error reading OldPrivValidator from %v: %v\n", oldPVPath, err)
	}
	logger.Info("Upgrading PrivValidator file",
		"old", oldPVPath,
		"newKey", newPVKeyPath,
		"newState", newPVStatePath,
	)
	oldPV.Upgrade(newPVKeyPath, newPVStatePath)
	return nil
}
