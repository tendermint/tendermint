package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/crypto"

	tmjson "github.com/tendermint/tendermint/libs/json"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/privval"
)

// ShowValidatorCmd adds capabilities for showing the validator info.
var ShowValidatorCmd = &cobra.Command{
	Use:     "show-validator",
	Aliases: []string{"show_validator"},
	Short:   "Show this node's validator info",
	RunE:    showValidator,
	PreRun:  deprecateSnakeCase,
}

func showValidator(cmd *cobra.Command, args []string) error {
	keyFilePath := config.PrivValidatorKeyFile()
	if !tmos.FileExists(keyFilePath) {
		return fmt.Errorf("private validator file %s does not exist", keyFilePath)
	}

	pv := privval.LoadFilePV(keyFilePath, config.PrivValidatorStateFile())

	pubKey, err := pv.GetPubKey(crypto.QuorumHash{})
	if err != nil {
		return fmt.Errorf("can't get pubkey in show validator: %w", err)
	}

	bz, err := tmjson.Marshal(pubKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private validator pubkey: %w", err)
	}

	fmt.Println(string(bz))
	return nil
}
