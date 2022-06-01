package commands

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
)

var (
	genesisHash []byte
)

// AddNodeFlags exposes some common configuration options from conf in the flag
// set for cmd. This is a convenience for commands embedding a Tendermint node.
func AddNodeFlags(cmd *cobra.Command, conf *cfg.Config) {
	// bind flags
	cmd.Flags().String("moniker", conf.Moniker, "node name")

	// mode flags
	cmd.Flags().String("mode", conf.Mode, "node mode (full | validator | seed)")

	// priv val flags
	cmd.Flags().String(
		"priv-validator-laddr",
		conf.PrivValidator.ListenAddr,
		"socket address to listen on for connections from external priv-validator process")

	// node flags

	cmd.Flags().BytesHexVar(
		&genesisHash,
		"genesis-hash",
		[]byte{},
		"optional SHA-256 hash of the genesis file")
	cmd.Flags().Int64("consensus.double-sign-check-height", conf.Consensus.DoubleSignCheckHeight,
		"how many blocks to look back to check existence of the node's "+
			"consensus votes before joining consensus")

	// abci flags
	cmd.Flags().String(
		"proxy-app",
		conf.ProxyApp,
		"proxy app address, or one of: 'kvstore',"+
			" 'persistent_kvstore', 'e2e' or 'noop' for local testing.")
	cmd.Flags().String("abci", conf.ABCI, "specify abci transport (socket | grpc)")

	// rpc flags
	cmd.Flags().String("rpc.laddr", conf.RPC.ListenAddress, "RPC listen address. Port required")
	cmd.Flags().Bool("rpc.unsafe", conf.RPC.Unsafe, "enabled unsafe rpc methods")
	cmd.Flags().String("rpc.pprof-laddr", conf.RPC.PprofListenAddress, "pprof listen address (https://golang.org/pkg/net/http/pprof)")

	// p2p flags
	cmd.Flags().String(
		"p2p.laddr",
		conf.P2P.ListenAddress,
		"node listen address. (0.0.0.0:0 means any interface, any port)")
	cmd.Flags().String("p2p.persistent-peers", conf.P2P.PersistentPeers, "comma-delimited ID@host:port persistent peers")
	cmd.Flags().Bool("p2p.upnp", conf.P2P.UPNP, "enable/disable UPNP port forwarding")
	cmd.Flags().Bool("p2p.pex", conf.P2P.PexReactor, "enable/disable Peer-Exchange")
	cmd.Flags().String("p2p.private-peer-ids", conf.P2P.PrivatePeerIDs, "comma-delimited private peer IDs")

	// consensus flags
	cmd.Flags().Bool(
		"consensus.create-empty-blocks",
		conf.Consensus.CreateEmptyBlocks,
		"set this to false to only produce blocks when there are txs or when the AppHash changes")
	cmd.Flags().String(
		"consensus.create-empty-blocks-interval",
		conf.Consensus.CreateEmptyBlocksInterval.String(),
		"the possible interval between empty blocks")

	addDBFlags(cmd, conf)
}

func addDBFlags(cmd *cobra.Command, conf *cfg.Config) {
	cmd.Flags().String(
		"db-backend",
		conf.DBBackend,
		"database backend: goleveldb | cleveldb | boltdb | rocksdb | badgerdb")
	cmd.Flags().String(
		"db-dir",
		conf.DBPath,
		"database directory")
}

// NewRunNodeCmd returns the command that allows the CLI to start a node.
// It can be used with a custom PrivValidator and in-process ABCI application.
func NewRunNodeCmd(nodeProvider cfg.ServiceProvider, conf *cfg.Config, logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"node", "run"},
		Short:   "Run the tendermint node",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkGenesisHash(conf); err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			n, err := nodeProvider(ctx, conf, logger)
			if err != nil {
				return fmt.Errorf("failed to create node: %w", err)
			}

			if err := n.Start(ctx); err != nil {
				return fmt.Errorf("failed to start node: %w", err)
			}

			logger.Info("started node", "chain", conf.ChainID())

			<-ctx.Done()
			return nil
		},
	}

	AddNodeFlags(cmd, conf)
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
