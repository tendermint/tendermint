package commands

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/privval"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/cmd/remote_val_harness/internal"
)

var (
	flagTendermintHome string
	flagOutputPath     string
)

func NewExtractKeyCmd() *cobra.Command {
	extractKeyCmd := &cobra.Command{
		Use:   "extract_key",
		Short: "Extracts a local Tendermint private validator key for testing",
		Long: `
Extracts a key from a local Tendermint validator instance for use in
integration testing with a remote signer. This requires valid local
priv_validator_key.json and priv_validator_state.json files.
`,
		Run: func(cmd *cobra.Command, args []string) {
			flagTendermintHome, err := internal.ExpandPath(flagTendermintHome)
			if err != nil {
				logger.Error("Failed to expand path", "tmhome", flagTendermintHome, "err", err)
				os.Exit(1)
			}
			flagOutputPath, err := internal.ExpandPath(flagOutputPath)
			if err != nil {
				logger.Error("Failed to expand path", "output", flagOutputPath, "err", err)
				os.Exit(2)
			}
			keyFile := filepath.Join(flagTendermintHome, "config", "priv_validator_key.json")
			stateFile := filepath.Join(flagTendermintHome, "data", "priv_validator_state.json")
			fpv := privval.LoadFilePV(keyFile, stateFile)
			pkb := [64]byte(fpv.Key.PrivKey.(ed25519.PrivKeyEd25519))
			err = ioutil.WriteFile(flagOutputPath, pkb[:32], 0644)
			if err != nil {
				logger.Error("Failed to write private key", "output", flagOutputPath, "err", err)
				os.Exit(3)
			}
			logger.Info("Successfully wrote private key", "output", flagOutputPath)
		},
	}

	extractKeyCmd.PersistentFlags().StringVar(&flagTendermintHome, "tmhome", "~/.tendermint", "Tendermint home directory")
	extractKeyCmd.PersistentFlags().StringVar(&flagOutputPath, "output", "./signing.key", "where to write the output key")

	return extractKeyCmd
}
