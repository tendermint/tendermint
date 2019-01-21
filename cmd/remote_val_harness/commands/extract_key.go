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
			tmhome, err := internal.ExpandPath(flagTendermintHome)
			if err != nil {
				logger.Info("Failed to expand path, using supplied path as-is", "tmhome", flagTendermintHome, "err", err)
				tmhome = flagTendermintHome
			}
			outputPath, err := internal.ExpandPath(flagOutputPath)
			if err != nil {
				logger.Info("Failed to expand path, using supplied path as-is", "output", flagOutputPath, "err", err)
				outputPath = flagOutputPath
			}
			keyFile := filepath.Join(tmhome, "config", "priv_validator_key.json")
			stateFile := filepath.Join(tmhome, "data", "priv_validator_state.json")
			fpv := privval.LoadFilePV(keyFile, stateFile)
			pkb := [64]byte(fpv.Key.PrivKey.(ed25519.PrivKeyEd25519))
			err = ioutil.WriteFile(outputPath, pkb[:32], 0644)
			if err != nil {
				logger.Info("Failed to write private key", "output", outputPath, "err", err)
				os.Exit(1)
			}
			logger.Info("Successfully wrote private key", "output", outputPath)
		},
	}

	extractKeyCmd.PersistentFlags().StringVar(&flagTendermintHome, "tmhome", "~/.tendermint", "Tendermint home directory")
	extractKeyCmd.PersistentFlags().StringVar(&flagOutputPath, "output", "./signing.key", "where to write the output key")

	return extractKeyCmd
}
