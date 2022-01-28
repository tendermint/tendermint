package main

import (
	"context"

	cmd "github.com/tendermint/tendermint/cmd/tendermint/commands"
	"github.com/tendermint/tendermint/cmd/tendermint/commands/debug"
	"github.com/tendermint/tendermint/libs/cli"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/node"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conf, err := cmd.ParseConfig()
	if err != nil {
		panic(err)
	}

	logger, err := log.NewDefaultLogger(conf.LogFormat, conf.LogLevel)
	if err != nil {
		panic(err)
	}

	rcmd := cmd.RootCommand(conf, logger)
	rcmd.AddCommand(
		cmd.MakeGenValidatorCommand(),
		cmd.MakeReindexEventCommand(conf, logger),
		cmd.MakeInitFilesCommand(conf, logger),
		cmd.MakeLightCommand(conf, logger),
		cmd.MakeReplayCommand(conf, logger),
		cmd.MakeReplayConsoleCommand(conf, logger),
		cmd.MakeResetAllCommand(conf, logger),
		cmd.MakeResetPrivateValidatorCommand(conf, logger),
		cmd.MakeShowValidatorCommand(conf, logger),
		cmd.MakeTestnetFilesCommand(conf, logger),
		cmd.MakeShowNodeIDCommand(conf),
		cmd.GenNodeKeyCmd,
		cmd.VersionCmd,
		cmd.MakeInspectCommand(conf, logger),
		cmd.MakeRollbackStateCommand(conf),
		cmd.MakeKeyMigrateCommand(conf, logger),
		debug.DebugCmd,
		cli.NewCompletionCmd(rcmd, true),
	)

	// NOTE:
	// Users wishing to:
	//	* Use an external signer for their validators
	//	* Supply an in-proc abci app
	//	* Supply a genesis doc file from another source
	//	* Provide their own DB implementation
	// can copy this file and use something other than the
	// node.NewDefault function
	nodeFunc := node.NewDefault

	// Create & start node
	rcmd.AddCommand(cmd.NewRunNodeCmd(nodeFunc, conf, logger))

	if err := rcmd.ExecuteContext(ctx); err != nil {
		panic(err)
	}
}
