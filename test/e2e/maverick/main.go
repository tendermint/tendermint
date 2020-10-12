package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cmd "github.com/tendermint/tendermint/cmd/tendermint/commands"
	"github.com/tendermint/tendermint/cmd/tendermint/commands/debug"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/cli"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	cs "github.com/tendermint/tendermint/test/e2e/maverick/consensus"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

var (
	config       = cfg.DefaultConfig()
	logger       = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	behaviorFlag = ""
)

// BehaviorList encompasses a list of all possible behaviors
var BehaviorList = map[string]cs.Behavior{
	"double-prevote": cs.NewDoublePrevoteBehavior(),
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
	Long: "Tendermint Maverick Node for testing with faulty consensus behaviors in a testnet. Contains " +
		"all the functionality of a normal node but custom behaviors can be injected when running the node " +
		"through a flag. See maverick node --help for how the behavior flag is constructured",
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
		ListBehaviorCmd,
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
			return startNode(config, logger, behaviorFlag)
		},
	}

	cmd.AddNodeFlags(nodeCmd)

	// Create & start node
	rootCmd.AddCommand(nodeCmd)

	// add special flag for behaviors
	nodeCmd.Flags().StringVar(
		&behaviorFlag,
		"behaviors",
		"",
		"Select the behaviors of the node (comma-separated, no spaces in between): \n"+
			"e.g. --behavior double-prevote,3\n"+
			"You can also have multiple behaviors: e.g. double-prevote,3,no-vote,5")

	cmd := cli.PrepareBaseCmd(rootCmd, "TM", os.ExpandEnv(filepath.Join("$HOME", ".maverick")))
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func startNode(config *cfg.Config, logger log.Logger, behaviorFlag string) error {
	fmt.Printf("behavior string: %s", behaviorFlag)
	behaviors, err := ParseBehaviors(behaviorFlag)
	if err != nil {
		return err
	}

	node, err := DefaultNewNode(config, logger, behaviors)
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

func ParseBehaviors(str string) (map[int64]cs.Behavior, error) {
	// check if string is empty in which case we run a normal node
	var behaviors = make(map[int64]cs.Behavior)
	if str == "" {
		return behaviors, nil
	}
	strs := strings.Split(str, ",")
	if len(strs)%2 != 0 {
		return behaviors, errors.New("missing either height or behavior name in the behavior flag")
	}
OUTER_LOOP:
	for i := 0; i < len(strs); i += 2 {
		height, err := strconv.ParseInt(strs[i+1], 10, 64)
		if err != nil {
			return behaviors, fmt.Errorf("failed to parse behavior height: %w", err)
		}
		for key, behavior := range BehaviorList {
			if key == strs[i] {
				behaviors[height] = behavior
				continue OUTER_LOOP
			}
		}
		return behaviors, fmt.Errorf("received unknown behavior: %s. Did you forget to add it?", strs[i])
	}

	return behaviors, nil
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

var ListBehaviorCmd = &cobra.Command{
	Use:   "behaviors",
	Short: "Lists possible behaviors",
	RunE:  listBehaviors,
}

func listBehaviors(cmd *cobra.Command, args []string) error {
	str := "Currently registered behaviors: \n"
	for key := range BehaviorList {
		str += fmt.Sprintf("- %s\n", key)
	}
	fmt.Printf(str)
	return nil
}
