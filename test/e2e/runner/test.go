package main

import (
	"context"
	"os"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/test/e2e/pkg/exec"
)

// Test runs test cases under tests/
func Test(testnet *e2e.Testnet, ifd e2e.InfrastructureData) error {
	logger.Info("Running tests in ./tests/...")

	err := os.Setenv("E2E_MANIFEST", testnet.File)
	if err != nil {
		return err
	}
	if p := ifd.Path(); p != "" {
		err = os.Setenv("INFRASTRUCTURE_DATA", p)
		if err != nil {
			return err
		}
	}
	err = os.Setenv("INFRASTRUCTURE_TYPE", ifd.Provider)
	if err != nil {
		return err
	}

	return exec.CommandVerbose(context.Background(), "go", "test", "-count", "1", "./tests/...")
}
