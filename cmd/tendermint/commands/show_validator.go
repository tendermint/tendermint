package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/jsontypes"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/privval"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
)

// MakeShowValidatorCommand constructs a command to show the validator info.
func MakeShowValidatorCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "show-validator",
		Short: "Show this node's validator info",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				pubKey crypto.PubKey
				err    error
				bctx   = cmd.Context()
			)
			//TODO: remove once gRPC is the only supported protocol
			protocol, _ := tmnet.ProtocolAndAddress(conf.PrivValidator.ListenAddr)
			switch protocol {
			case "grpc":
				pvsc, err := tmgrpc.DialRemoteSigner(
					bctx,
					conf.PrivValidator,
					conf.ChainID(),
					logger,
					conf.Instrumentation.Prometheus,
				)
				if err != nil {
					return fmt.Errorf("can't connect to remote validator %w", err)
				}

				ctx, cancel := context.WithTimeout(bctx, ctxTimeout)
				defer cancel()

				pubKey, err = pvsc.GetPubKey(ctx)
				if err != nil {
					return fmt.Errorf("can't get pubkey: %w", err)
				}
			default:

				keyFilePath := conf.PrivValidator.KeyFile()
				if !tmos.FileExists(keyFilePath) {
					return fmt.Errorf("private validator file %s does not exist", keyFilePath)
				}

				pv, err := privval.LoadFilePV(keyFilePath, conf.PrivValidator.StateFile())
				if err != nil {
					return err
				}

				ctx, cancel := context.WithTimeout(bctx, ctxTimeout)
				defer cancel()

				pubKey, err = pv.GetPubKey(ctx)
				if err != nil {
					return fmt.Errorf("can't get pubkey: %w", err)
				}
			}

			bz, err := jsontypes.Marshal(pubKey)
			if err != nil {
				return fmt.Errorf("failed to marshal private validator pubkey: %w", err)
			}

			fmt.Println(string(bz))
			return nil
		},
	}

}
