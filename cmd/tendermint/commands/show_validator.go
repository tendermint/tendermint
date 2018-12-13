package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/privval"
)

func init() {
	ShowValidatorCmd.Flags().String("priv_validator_laddr", config.PrivValidatorListenAddr, "Socket address to listen on for connections from external priv_validator process")
}

// ShowValidatorCmd adds capabilities for showing the validator info.
var ShowValidatorCmd = &cobra.Command{
	Use:   "show_validator",
	Short: "Show this node's validator info",
	Run:   showValidator,
}

func showValidator(cmd *cobra.Command, args []string) {
	var pubKey crypto.PubKey
	if config.PrivValidatorListenAddr != "" {
		// If an address is provided, listen on the socket for a connection from an
		// external signing process and request the public key from there.
		privValidator, err := node.CreateAndStartPrivValidatorSocketClient(config.PrivValidatorListenAddr, logger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get private validator's public key: %v", err)
			os.Exit(-1)
		}
		if pvsc, ok := privValidator.(cmn.Service); ok {
			if err := pvsc.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get stop private validator client: %v", err)
			}
		}
		pubKey = privValidator.GetPubKey()
	} else {
		privValidator := privval.LoadOrGenFilePV(config.PrivValidatorFile())
		pubKey = privValidator.GetPubKey()
	}
	pubKeyJSONBytes, _ := cdc.MarshalJSON(pubKey)
	fmt.Println(string(pubKeyJSONBytes))
}
