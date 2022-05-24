package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/scripts/keymigrate"
	"github.com/tendermint/tendermint/scripts/scmigrate"
)

func MakeKeyMigrateCommand(conf *cfg.Config, logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key-migrate",
		Short: "Run Database key migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			contexts := []string{
				// this is ordered to put the
				// (presumably) biggest/most important
				// subsets first.
				"tx_index",
				"peerstore",
				"light",
				"blockstore",
				"evidence",
				"state",
			}

			for idx, dbctx := range contexts {
				logger.Info("beginning a key migration",
					"dbctx", dbctx,
					"num", idx+1,
					"total", len(contexts),
				)

				dbFrom, err := cfg.DefaultDBProvider(&cfg.DBContext{
					ID:       dbctx,
					Config:   conf,
					IsLegacy: true,
				})

				if err != nil {
					return fmt.Errorf("constructing source database handle: %w", err)
				}

				dbTo, err := cfg.DefaultDBProvider(&cfg.DBContext{
					ID:     fmt.Sprint(dbctx, 0),
					Config: conf,
				})

				if err != nil {
					return fmt.Errorf("constructing target database handle: %w", err)
				}

				if err = keymigrate.Migrate(ctx, keymigrate.MigrateDB{
					From: dbFrom,
					To:   dbTo,
				}); err != nil {
					return fmt.Errorf("running migration for context %q: %w",
						dbctx, err)
				}

				if dbctx == "blockstore" {
					if err := scmigrate.Migrate(ctx, scmigrate.MigrateDB{
						From: dbFrom,
						To:   dbTo,
					}); err != nil {
						return fmt.Errorf("running seen commit migration: %w", err)

					}
				}
			}

			logger.Info("completed database migration successfully")

			return nil
		},
	}

	// allow database info to be overridden via cli
	addDBFlags(cmd, conf)

	return cmd
}
