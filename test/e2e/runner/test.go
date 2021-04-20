package main

import (
	"os"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Test runs test cases under tests/
func Test(testnet *e2e.Testnet) error {
	logger.Info("Running tests in ./tests/...")

	err := os.Setenv("E2E_MANIFEST", testnet.File)
	if err != nil {
		return err
	}

	return execVerbose("go", "test", "-count", "1", "./tests/...")
}
