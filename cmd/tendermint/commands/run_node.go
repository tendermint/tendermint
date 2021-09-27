package commands

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	cfg "github.com/tendermint/tendermint/config"
	tmos "github.com/tendermint/tendermint/libs/os"
)

var (
	genesisHash []byte
)

// AddNodeFlags exposes some common configuration options on the command-line
// These are exposed for convenience of commands embedding a tendermint node
func AddNodeFlags(cmd *cobra.Command) {
	// bind flags
	cmd.Flags().String("moniker", config.Moniker, "node name")

	// mode flags
	cmd.Flags().String("mode", config.Mode, "node mode (full | validator | seed)")

	// priv val flags
	cmd.Flags().String(
		"priv-validator-laddr",
		config.PrivValidator.ListenAddr,
		"socket address to listen on for connections from external priv-validator process")

	// node flags
	cmd.Flags().Bool("blocksync.enable", config.BlockSync.Enable, "enable fast blockchain syncing")

	// TODO (https://github.com/tendermint/tendermint/issues/6908): remove this check after the v0.35 release cycle
	// This check was added to give users an upgrade prompt to use the new flag for syncing.
	//
	// The pflag package does not have a native way to print a depcrecation warning
	// and return an error. This logic was added to print a deprecation message to the user
	// and then crash if the user attempts to use the old --fast-sync flag.
	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.Func("fast-sync", "deprecated",
		func(string) error {
			return errors.New("--fast-sync has been deprecated, please use --blocksync.enable")
		})
	cmd.Flags().AddGoFlagSet(fs)

	cmd.Flags().MarkHidden("fast-sync") //nolint:errcheck
	cmd.Flags().BytesHexVar(
		&genesisHash,
		"genesis-hash",
		[]byte{},
		"optional SHA-256 hash of the genesis file")
	cmd.Flags().Int64("consensus.double-sign-check-height", config.Consensus.DoubleSignCheckHeight,
		"how many blocks to look back to check existence of the node's "+
			"consensus votes before joining consensus")

	// abci flags
	cmd.Flags().String(
		"proxy-app",
		config.ProxyApp,
		"proxy app address, or one of: 'kvstore',"+
			" 'persistent_kvstore', 'e2e' or 'noop' for local testing.")
	cmd.Flags().String("abci", config.ABCI, "specify abci transport (socket | grpc)")

	// rpc flags
	cmd.Flags().String("rpc.laddr", config.RPC.ListenAddress, "RPC listen address. Port required")
	cmd.Flags().String(
		"rpc.grpc-laddr",
		config.RPC.GRPCListenAddress,
		"GRPC listen address (BroadcastTx only). Port required")
	cmd.Flags().Bool("rpc.unsafe", config.RPC.Unsafe, "enabled unsafe rpc methods")
	cmd.Flags().String("rpc.pprof-laddr", config.RPC.PprofListenAddress, "pprof listen address (https://golang.org/pkg/net/http/pprof)")

	// p2p flags
	cmd.Flags().String(
		"p2p.laddr",
		config.P2P.ListenAddress,
		"node listen address. (0.0.0.0:0 means any interface, any port)")
	cmd.Flags().String("p2p.seeds", config.P2P.Seeds, "comma-delimited ID@host:port seed nodes")
	cmd.Flags().String("p2p.persistent-peers", config.P2P.PersistentPeers, "comma-delimited ID@host:port persistent peers")
	cmd.Flags().String("p2p.unconditional-peer-ids",
		config.P2P.UnconditionalPeerIDs, "comma-delimited IDs of unconditional peers")
	cmd.Flags().Bool("p2p.upnp", config.P2P.UPNP, "enable/disable UPNP port forwarding")
	cmd.Flags().Bool("p2p.pex", config.P2P.PexReactor, "enable/disable Peer-Exchange")
	cmd.Flags().String("p2p.private-peer-ids", config.P2P.PrivatePeerIDs, "comma-delimited private peer IDs")

	// consensus flags
	cmd.Flags().Bool(
		"consensus.create-empty-blocks",
		config.Consensus.CreateEmptyBlocks,
		"set this to false to only produce blocks when there are txs or when the AppHash changes")
	cmd.Flags().String(
		"consensus.create-empty-blocks-interval",
		config.Consensus.CreateEmptyBlocksInterval.String(),
		"the possible interval between empty blocks")

	addDBFlags(cmd)
}

func addDBFlags(cmd *cobra.Command) {
	cmd.Flags().String(
		"db-backend",
		config.DBBackend,
		"database backend: goleveldb | cleveldb | boltdb | rocksdb | badgerdb")
	cmd.Flags().String(
		"db-dir",
		config.DBPath,
		"database directory")
}

// NewRunNodeCmd returns the command that allows the CLI to start a node.
// It can be used with a custom PrivValidator and in-process ABCI application.
func NewRunNodeCmd(nodeProvider cfg.ServiceProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"node", "run"},
		Short:   "Run the tendermint node",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkGenesisHash(config); err != nil {
				return err
			}

			n, err := nodeProvider(config, logger)
			if err != nil {
				return fmt.Errorf("failed to create node: %w", err)
			}

			if err := n.Start(); err != nil {
				return fmt.Errorf("failed to start node: %w", err)
			}

			logger.Info("started node", "node", n.String())

			// Stop upon receiving SIGTERM or CTRL-C.
			tmos.TrapSignal(logger, func() {
				if n.IsRunning() {
					if err := n.Stop(); err != nil {
						logger.Error("unable to stop the node", "error", err)
					}
				}
			})

			// Run forever.
			select {}
		},
	}

	AddNodeFlags(cmd)
	return cmd
}

func checkGenesisHash(config *cfg.Config) error {
	if len(genesisHash) == 0 || config.Genesis == "" {
		return nil
	}

	// Calculate SHA-256 hash of the genesis file.
	f, err := os.Open(config.GenesisFile())
	if err != nil {
		return fmt.Errorf("can't open genesis file: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("error when hashing genesis file: %w", err)
	}
	actualHash := h.Sum(nil)

	// Compare with the flag.
	if !bytes.Equal(genesisHash, actualHash) {
		return fmt.Errorf(
			"--genesis-hash=%X does not match %s hash: %X",
			genesisHash, config.GenesisFile(), actualHash)
	}

	return nil
}
