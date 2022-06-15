package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/tendermint/tendermint/libs/log"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"

	"github.com/tendermint/tendermint/scripts/keymigrate"
	"github.com/tendermint/tendermint/scripts/scmigrate"

	"github.com/tendermint/tendermint/version"
)

func Upgrade(ctx context.Context, testnet *e2e.Testnet, logger log.Logger) error {
	if err := Cleanup(logger, testnet); err != nil {
		return err
	}

	if err := Setup(logger, testnet); err != nil {
		return err
	}

	r := rand.New(rand.NewSource(randomSeed)) // nolint: gosec

	chLoadResult := make(chan error)

	lctx, loadCancel := context.WithCancel(ctx)
	defer loadCancel()
	go func() {
		chLoadResult <- Load(lctx, logger, r, testnet)
	}()

	if err := Start(ctx, logger, testnet); err != nil {
		return err
	}

	if err := Wait(ctx, logger, testnet, 10); err != nil { // allow some txs to go through
		return err
	}

	loadCancel()
	if err := <-chLoadResult; err != nil {
		return fmt.Errorf("transaction load failed: %w", err)
	}

	// stop the network
	if err := execCompose(testnet.Dir, "down"); err != nil {
		return err
	}

	if err := Migrate(ctx, logger, testnet); err != nil {
		return err
	}

	if err := Start(ctx, logger, testnet); err != nil {
		return err
	}

	return nil
}

func Migrate(ctx context.Context, logger log.Logger, testnet *e2e.Testnet) error {
	stores := []string{
		"tx_index",
		"light",
		"blockstore",
		"state",
		"evidence",
	}
	for _, node := range testnet.Nodes {
		if strings.HasPrefix(testnet.Version, "v0.34") && version.TMVersion > "v0.35" {
			for _, store := range stores {
				db, err := node.DB(store)
				if err != nil {
					return fmt.Errorf("migrating db: %w", err)
				}

				if err = keymigrate.Migrate(ctx, db); err != nil {
					return fmt.Errorf("running migration for context %q: %w",
						store, err)
				}

				if store == "blockstore" {
					if err := scmigrate.Migrate(ctx, db); err != nil {
						return fmt.Errorf("running seen commit migration: %w", err)
					}
				}
			}
		}

	}
	return nil
}
