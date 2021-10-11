package commands

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	cfg "github.com/tendermint/tendermint/config"
	tmos "github.com/tendermint/tendermint/libs/os"
	nm "github.com/tendermint/tendermint/node"
)

var (
	genesisHash []byte
)

// AddNodeFlags exposes some common configuration options on the command-line
// These are exposed for convenience of commands embedding a tendermint node
func AddNodeFlags(cmd *cobra.Command) {
	// bind flags
	cmd.Flags().String("moniker", config.Moniker, "Node Name")

	// priv val flags
	cmd.Flags().String(
		"priv_validator_laddr",
		config.PrivValidatorListenAddr,
		"Socket address to listen on for connections from external priv_validator process")

	// node flags
	cmd.Flags().Bool("fast_sync", config.FastSyncMode, "Fast blockchain syncing")
	cmd.Flags().BytesHexVar(
		&genesisHash,
		"genesis_hash",
		[]byte{},
		"Optional SHA-256 hash of the genesis file")

	// abci flags
	cmd.Flags().String(
		"proxy_app",
		config.ProxyApp,
		"Proxy app address, or one of: 'kvstore',"+
			" 'persistent_kvstore',"+
			" 'counter',"+
			" 'counter_serial' or 'noop' for local testing.")
	cmd.Flags().String("abci", config.ABCI, "Specify abci transport (socket | grpc)")

	// rpc flags
	cmd.Flags().String("rpc.laddr", config.RPC.ListenAddress, "RPC listen address. Port required")
	cmd.Flags().String(
		"rpc.grpc_laddr",
		config.RPC.GRPCListenAddress,
		"GRPC listen address (BroadcastTx only). Port required")
	cmd.Flags().Bool("rpc.unsafe", config.RPC.Unsafe, "Enabled unsafe rpc methods")

	// p2p flags
	cmd.Flags().String(
		"p2p.laddr",
		config.P2P.ListenAddress,
		"Node listen address. (0.0.0.0:0 means any interface, any port)")
	cmd.Flags().String("p2p.seeds", config.P2P.Seeds, "Comma-delimited ID@host:port seed nodes")
	cmd.Flags().String("p2p.persistent_peers", config.P2P.PersistentPeers, "Comma-delimited ID@host:port persistent peers")
	cmd.Flags().String("p2p.unconditional_peer_ids",
		config.P2P.UnconditionalPeerIDs, "Comma-delimited IDs of unconditional peers")
	cmd.Flags().Bool("p2p.upnp", config.P2P.UPNP, "Enable/disable UPNP port forwarding")
	cmd.Flags().Bool("p2p.pex", config.P2P.PexReactor, "Enable/disable Peer-Exchange")
	cmd.Flags().Bool("p2p.seed_mode", config.P2P.SeedMode, "Enable/disable seed mode")
	cmd.Flags().String("p2p.private_peer_ids", config.P2P.PrivatePeerIDs, "Comma-delimited private peer IDs")

	// consensus flags
	cmd.Flags().Bool(
		"consensus.create_empty_blocks",
		config.Consensus.CreateEmptyBlocks,
		"Set this to false to only produce blocks when there are txs or when the AppHash changes")
	cmd.Flags().String(
		"consensus.create_empty_blocks_interval",
		config.Consensus.CreateEmptyBlocksInterval.String(),
		"The possible interval between empty blocks")
	// mempool flags
	cmd.Flags().Bool(
		"mempool.sealed",
		config.Mempool.Sealed,
		"Set this to true only for debug mode",
	)
	cmd.Flags().MarkHidden("mempool.sealed")
	// mempool flags
	cmd.Flags().Bool(
		"mempool.recheck",
		config.Mempool.Recheck,
		"Enable recheck of txs remain pending in mempool",
	)
	cmd.Flags().Int64(
		"mempool.force_recheck_gap",
		config.Mempool.ForceRecheckGap,
		"The interval to force recheck of txs remain pending in mempool",
	)
	cmd.Flags().Int(
		"mempool.size",
		config.Mempool.Size,
		"Maximum number of transactions in the mempool",
	)
	cmd.Flags().Int64(
		"mempool.max_tx_num_per_block",
		config.Mempool.MaxTxNumPerBlock,
		"Maximum number of transactions in a block",
	)
	cmd.Flags().Int64(
		"mempool.max_gas_used_per_block",
		config.Mempool.MaxGasUsedPerBlock,
		"Maximum gas used of transactions in a block",
	)
	cmd.Flags().Bool(
		"mempool.sort_tx_by_gp",
		config.Mempool.SortTxByGp,
		"Sort tx by gas price in mempool",
	)
	cmd.Flags().Uint64(
		"mempool.tx_price_bump",
		config.Mempool.TxPriceBump,
		"Minimum price bump percentage to replace an already existing transaction with same nonce",
	)
	cmd.Flags().Bool(
		"mempool.enable_pending_pool",
		config.Mempool.EnablePendingPool,
		"Enable pending pool to cache txs with discontinuous nonce",
	)
	cmd.Flags().Int(
		"mempool.pending_pool_size",
		config.Mempool.PendingPoolSize,
		"Maximum number of transactions in the pending pool",
	)
	cmd.Flags().Int(
		"mempool.pending_pool_period",
		config.Mempool.PendingPoolPeriod,
		"The time period in second to wait to consume the pending pool txs when the mempool is full ",
	)
	cmd.Flags().Int(
		"mempool.pending_pool_reserve_blocks",
		config.Mempool.PendingPoolReserveBlocks,
		"The number of blocks that the address is allowed to reserve in the pending pool",
	)
	cmd.Flags().Int(
		"mempool.pending_pool_max_tx_per_address",
		config.Mempool.PendingPoolMaxTxPerAddress,
		"Maximum number of transactions per address in the pending pool",
	)

	// db flags
	cmd.Flags().String(
		"db_backend",
		config.DBBackend,
		"Database backend: goleveldb | cleveldb | boltdb | rocksdb")
	cmd.Flags().String(
		"db_dir",
		config.DBPath,
		"Database directory")

	addMoreFlags(cmd)
}

// NewRunNodeCmd returns the command that allows the CLI to start a node.
// It can be used with a custom PrivValidator and in-process ABCI application.
func NewRunNodeCmd(nodeProvider nm.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Run the tendermint node",
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

			logger.Info("Started node", "nodeInfo", n.Switch().NodeInfo())

			// Stop upon receiving SIGTERM or CTRL-C.
			tmos.TrapSignal(logger, func() {
				if n.IsRunning() {
					n.Stop()
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
		return errors.Wrap(err, "can't open genesis file")
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return errors.Wrap(err, "error when hashing genesis file")
	}
	actualHash := h.Sum(nil)

	// Compare with the flag.
	if !bytes.Equal(genesisHash, actualHash) {
		return errors.Errorf(
			"--genesis_hash=%X does not match %s hash: %X",
			genesisHash, config.GenesisFile(), actualHash)
	}

	return nil
}
