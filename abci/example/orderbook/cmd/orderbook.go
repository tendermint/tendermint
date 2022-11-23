package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/tendermint/tendermint/abci/example/orderbook"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func main() {
	NewCLI().Run()
}

type CLI struct {
	root *cobra.Command
	config *cfg.Config
}

func NewCLI() *CLI {
	cli := &CLI{}
	cli.root = &cobra.Command{
		Use:   "orderbook",
		Short: "orderbook abci++ example",
	}
	cli.root.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "initialize the file system for an orderbook node",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			
			viper.AddConfigPath(filepath.Join(root, "config"))
			viper.SetConfigName("config")

			if err := viper.ReadInConfig(); err != nil {
				// return err
			}

			config := cfg.DefaultConfig()

			if err := viper.Unmarshal(config); err != nil {
				return err
			}

			config.SetRoot(root)
			cli.config = config
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			privValKeyFile := cli.config.PrivValidatorKeyFile()
			privValStateFile := cli.config.PrivValidatorStateFile()
			var pv *privval.FilePV
			if tmos.FileExists(privValKeyFile) {
				pv = privval.LoadFilePV(privValKeyFile, privValStateFile)
				fmt.Print("found private validator", "keyFile", privValKeyFile,
					"stateFile", privValStateFile)
			} else {
				pv = privval.GenFilePV(privValKeyFile, privValStateFile)
				pv.Save()
				fmt.Print("Generated private validator", "keyFile", privValKeyFile,
					"stateFile", privValStateFile)
			}

			nodeKeyFile := cli.config.NodeKeyFile()
			if tmos.FileExists(nodeKeyFile) {
				fmt.Print("Found node key", "path", nodeKeyFile)
			} else {
				if _, err := p2p.LoadOrGenNodeKey(nodeKeyFile); err != nil {
					return err
				}
				fmt.Print("Generated node key", "path", nodeKeyFile)
			}

			// genesis file
			genFile := cli.config.GenesisFile()
			if tmos.FileExists(genFile) {
				fmt.Print("Found genesis file", "path", genFile)
			} else {
				genDoc := types.GenesisDoc{
					ChainID:         fmt.Sprintf("orderbook-chain-%v", tmrand.Int()),
					GenesisTime:     tmtime.Now(),
					ConsensusParams: types.DefaultConsensusParams(),
				}
				pubKey, err := pv.GetPubKey()
				if err != nil {
					return fmt.Errorf("can't get pubkey: %w", err)
				}
				genDoc.Validators = []types.GenesisValidator{{
					Address: pubKey.Address(),
					PubKey:  pubKey,
					Power:   10,
				}}

				if err := genDoc.SaveAs(genFile); err != nil {
					return err
				}
				fmt.Print("Generated genesis file", "path", genFile)
			}

			return nil
		},
	})
	cli.root.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "runs an orderbook node",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbProvider := node.DefaultDBProvider
			appDB, err := dbProvider(&node.DBContext{"orderbook", cli.config})
			if err != nil {
				return err
			}
			app, err := orderbook.New(appDB)
			if err != nil {
				return err
			}

			nodeKey, err := p2p.LoadOrGenNodeKey(cli.config.NodeKeyFile())
			if err != nil {
				return fmt.Errorf("failed to load or gen node key %s: %w", cli.config.NodeKeyFile(), err)
			}

			logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
			n, err := node.NewNode(
				cli.config,
				privval.LoadOrGenFilePV(cli.config.PrivValidatorKeyFile(), cli.config.PrivValidatorStateFile()),
				nodeKey,
				proxy.NewLocalClientCreator(app),
				node.DefaultGenesisDocProviderFunc(cli.config),
				dbProvider,
				node.DefaultMetricsProvider(cli.config.Instrumentation),
				logger,
			)
			if err != nil {
				return err
			}

			if err := n.Start(); err != nil {
				return err
			}

			tmos.TrapSignal(logger, func() {
				if err := n.Stop(); err != nil {
					logger.Error("unable to stop the node", "error", err)
				}
			})

			return nil
		},
	})
	cli.root.AddCommand(&cobra.Command{
		Use:     "create-account [commodities...]",
		Short:   "creates a new account message and submits it to the chain",
		Example: "create-account 500BTC 10000USD",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	})
	cli.root.AddCommand(&cobra.Command{
		Use:     "create-pair buyers-denomination sellers-denomination",
		Short:   "creates a new pair message and submits it to the chain",
		Example: "create-pair BTC USD",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	})
	cli.root.AddCommand(&cobra.Command{
		Use:     "bid buying-commodity price",
		Short:   "creates a bid message and submits it to the chain",
		Example: "bid 10BTC 15000BTC/USD",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	})
	cli.root.AddCommand(&cobra.Command{
		Use:     "ask selling-commodity price",
		Short:   "creates an ask message and submits it to the chain",
		Example: "ask 5BTC 12000BTC/USD",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	})
	querySubcommand := &cobra.Command{
		Use:   "query",
		Short: "query the bal",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	querySubcommand.AddCommand(&cobra.Command{
		Use:   "account pubkey|id",
		Short: "query the balance of an account",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	})
	querySubcommand.AddCommand(&cobra.Command{
		Use:   "pairs",
		Short: "list all the trading pairs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	})
	querySubcommand.AddCommand(&cobra.Command{
		Use:     "orders pair",
		Short:   "list all current orders for a given pair",
		Example: "orders BTC/USD",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	})
	cli.root.AddCommand(querySubcommand)

	return cli
}

// Run runs the CLI.
func (cli *CLI) Run() {
	if err := cli.root.Execute(); err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}
