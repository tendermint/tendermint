package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cmd "github.com/tendermint/tendermint/cmd/tendermint/commands"
	"github.com/tendermint/tendermint/cmd/tendermint/commands/debug"
	cfg "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	"github.com/tendermint/tendermint/libs/cli"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	cs "github.com/tendermint/tendermint/test/e2e/maverick/consensus"
)

var (
	config       = cfg.DefaultConfig()
	logger       = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	behaviorFlag = ""
	heightFlag = 1
)

// Behaviors
//
// Add any new behaviors to this array and to the switch statement below in order for the maverick node instance
// to recognize the behavior

var (
	AllBehaviors = []cs.Behavior{
		&cs.DefaultBehavior{},
		&cs.EquivocationBehavior{},
	}
)

func getBehavior(name string) (cs.Behavior, error) {
	switch name {
	// Never remove the default behavior
	case "Default":
		return &cs.DefaultBehavior{}, nil
	case "Equivocation":
		return &cs.EquivocationBehavior{}, nil
	default:
		return  &cs.DefaultBehavior{}, fmt.Errorf("unable to recognize behavior have you added it: %s", name)
	}
}

func init() {
	registerFlagsRootCmd(RootCmd)
}

func registerFlagsRootCmd(command *cobra.Command) {
	command.PersistentFlags().String("log_level", config.LogLevel, "Log level")
}

func ParseConfig() (*cfg.Config, error) {
	conf := cfg.DefaultConfig()
	err := viper.Unmarshal(conf)
	if err != nil {
		return nil, err
	}
	conf.SetRoot(conf.RootDir)
	cfg.EnsureRoot(conf.RootDir)
	if err = conf.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("error in config file: %v", err)
	}
	return conf, err
}

// RootCmd is the root command for Tendermint core.
var RootCmd = &cobra.Command{
	Use:   "maverick",
	Short: "Tendermint Maverick Node",
	Long:  "Tendermint Maverick Node for testing with faulty consensus behavior in a testnet",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err = ParseConfig()
		if err != nil {
			return err
		}
		if config.LogFormat == cfg.LogFormatJSON {
			logger = log.NewTMJSONLogger(log.NewSyncWriter(os.Stdout))
		}
		logger, err = tmflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel())
		if err != nil {
			return err
		}
		if viper.GetBool(cli.TraceFlag) {
			logger = log.NewTracingLogger(logger)
		}
		logger = logger.With("module", "main")
		return nil
	},
}

func main() {
	rootCmd := RootCmd
	rootCmd.AddCommand(
		cmd.GenValidatorCmd,
		InitFilesCmd,
		cmd.ProbeUpnpCmd,
		cmd.ReplayCmd,
		cmd.ReplayConsoleCmd,
		cmd.ResetAllCmd,
		cmd.ResetPrivValidatorCmd,
		cmd.ShowValidatorCmd,
		cmd.ShowNodeIDCmd,
		cmd.GenNodeKeyCmd,
		cmd.VersionCmd,
		debug.DebugCmd,
		cli.NewCompletionCmd(rootCmd, true),
	)

	nodeCmd := &cobra.Command{
		Use:   "node",
		Short: "Run the maverick node",
		RunE: func(command *cobra.Command, args []string) error {
			return startNode(config, logger, behaviorFlag, heightFlag)
		},
	}

	cmd.AddNodeFlags(nodeCmd)

	// Create & start node
	rootCmd.AddCommand(nodeCmd)

	// add special flag for behaviors
	nodeCmd.Flags().String(
		"behavior",
		behaviorFlag,
		"Select a particular behavior for the node to execute (no behavior selected will mean the node will behave normally):"+
			" e.g. --behavior equivocation")

	nodeCmd.Flags().Int(
		"height",
		heightFlag,
		"Select the height to execute the behavior (in other heights the node behaves normally): "+
			" e.g. --height 3")

	cmd := cli.PrepareBaseCmd(rootCmd, "TM", os.ExpandEnv(filepath.Join("$HOME", ".maverick")))
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func startNode(config *cfg.Config, logger log.Logger, behaviorFlag string, heightFlag int) error {
	node, err := DefaultNewNode(config, logger, behaviorFlag, int64(heightFlag))
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	if err := node.Start(); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	logger.Info("Started node", "nodeInfo", node.Switch().NodeInfo())

	// Stop upon receiving SIGTERM or CTRL-C.
	tmos.TrapSignal(logger, func() {
		if node.IsRunning() {
			node.Stop()
		}
	})

	// Run forever.
	select {}
}

var InitFilesCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Tendermint",
	RunE:  initFiles,
}

func initFiles(cmd *cobra.Command, args []string) error {
	return initFilesWithConfig(config)
}

func initFilesWithConfig(config *cfg.Config) error {
	// private validator
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()
	var pv *FilePV
	if tmos.FileExists(privValKeyFile) {
		pv = LoadFilePV(privValKeyFile, privValStateFile)
		logger.Info("Found private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	} else {
		pv = GenFilePV(privValKeyFile, privValStateFile)
		pv.Save()
		logger.Info("Generated private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	}

	nodeKeyFile := config.NodeKeyFile()
	if tmos.FileExists(nodeKeyFile) {
		logger.Info("Found node key", "path", nodeKeyFile)
	} else {
		if _, err := p2p.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			return err
		}
		logger.Info("Generated node key", "path", nodeKeyFile)
	}

	// genesis file
	genFile := config.GenesisFile()
	if tmos.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		genDoc := types.GenesisDoc{
			ChainID:         fmt.Sprintf("test-chain-%v", tmrand.Str(6)),
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
		logger.Info("Generated genesis file", "path", genFile)
	}

	return nil
}

// var ListBehaviorCmd = &cobra.Command{
// 	Use:   "list",
// 	Short: "Lists possible behaviors",
// 	RunE:  listBehaviors,
// }

// func listBehaviors(cmd *cobra.Command, args []string) error {
// 	fmt.Prin
// } 
