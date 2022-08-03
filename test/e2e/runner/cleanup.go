package main

import (
	"context"
	"errors"
	"os"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/test/e2e/pkg/infra"
)

// Cleanup destroys all infrastructure and removes all generated testnet files.
func Cleanup(ctx context.Context, logger log.Logger, testnetDir string, ti infra.TestnetInfra) error {
	if testnetDir == "" {
		return errors.New("no testnet directory set")
	}

	if err := ti.Cleanup(ctx); err != nil {
		return err
	}

	_, err := os.Stat(testnetDir)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	logger.Info("Removing testnet", "directory", testnetDir)
	return os.RemoveAll(testnetDir)
}
