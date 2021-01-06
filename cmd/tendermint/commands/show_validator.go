package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/crypto"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmnet "github.com/tendermint/tendermint/libs/net"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/privval"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
)

// ShowValidatorCmd adds capabilities for showing the validator info.
var ShowValidatorCmd = &cobra.Command{
	Use:     "show-validator [chainID]",
	Aliases: []string{"show_validator"},
	Short:   "Show this node's validator info",
	Args:    cobra.ExactArgs(1),
	RunE:    showValidator,
	PreRun:  deprecateSnakeCase,
}

func showValidator(cmd *cobra.Command, args []string) error {
	var (
		pubKey crypto.PubKey
		err    error
	)

	//todo: remove once gRPC is the only supported protocol
	protocol, _ := tmnet.ProtocolAndAddress(config.PrivValidatorListenAddr)
	switch protocol {
	case "grpc":
		pvsc, err := tmgrpc.DialRemoteSigner(config, args[0], logger)
		if err != nil {
			return fmt.Errorf("can't instansiate connection: %w", err)
		}
		pubKey, err = pvsc.GetPubKey()
		if err != nil {
			return fmt.Errorf("can't get pubkey: %w", err)
		}
	default:

		keyFilePath := config.PrivValidatorKeyFile()
		if !tmos.FileExists(keyFilePath) {
			return fmt.Errorf("private validator file %s does not exist", keyFilePath)
		}

		pv := privval.LoadFilePV(keyFilePath, config.PrivValidatorStateFile())

		pubKey, err = pv.GetPubKey()
		if err != nil {
			return fmt.Errorf("can't get pubkey: %w", err)
		}
	}

	bz, err := tmjson.Marshal(pubKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private validator pubkey: %w", err)
	}

	fmt.Println(string(bz))
	return nil
}
