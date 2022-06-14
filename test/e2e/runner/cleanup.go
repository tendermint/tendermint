package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/tendermint/tendermint/libs/log"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/test/e2e/pkg/infra"
)

// Cleanup destroys all infrastructure and removes all generated testnet files.
func Cleanup(ctx context.Context, logger log.Logger, testnet *e2e.Testnet, provider infra.Provider) error {
	if testnet.Dir == "" {
		return errors.New("no testnet directory set")
	}

	if err := provider.Cleanup(ctx); err != nil {
		return err
	}

	_, err := os.Stat(testnet.Dir)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("Removing testnet directory %q", testnet.Dir))
	return os.RemoveAll(testnet.Dir)
}
