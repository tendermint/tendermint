package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/scripts/keymigrate"
	"github.com/tendermint/tendermint/scripts/scmigrate"
)

func MakeKeyMigrateCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key-migrate",
		Short: "Run Database key migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunDatabaseMigration(cmd.Context(), logger, conf)
		},
	}

	// allow database info to be overridden via cli
	addDBFlags(cmd, conf)

	return cmd
}

func RunDatabaseMigration(ctx context.Context, logger log.Logger, conf *config.Config) error {
	contexts := []string{
		// this is ordered to put
		// the more ephemeral tables first to
		// reduce the possibility of the
		// ephemeral data overwriting later data
		"tx_index",
		"peerstore",
		"light",
		"blockstore",
		"state",
		"evidence",
	}

	for idx, dbctx := range contexts {
		logger.Info("beginning a key migration",
			"dbctx", dbctx,
			"num", idx+1,
			"total", len(contexts),
		)

		db, err := config.DefaultDBProvider(&config.DBContext{
			ID:     dbctx,
			Config: conf,
		})

		if err != nil {
			return fmt.Errorf("constructing database handle: %w", err)
		}

		if err = keymigrate.Migrate(ctx, db); err != nil {
			return fmt.Errorf("running migration for context %q: %w",
				dbctx, err)
		}

		if dbctx == "blockstore" {
			if err := scmigrate.Migrate(ctx, db); err != nil {
				return fmt.Errorf("running seen commit migration: %w", err)

			}
		}
	}

	logger.Info("completed database migration successfully")

	return nil
}
