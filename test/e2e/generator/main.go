//nolint: gosec
package main

import (
	"context"
	"fmt"
	stdlog "log"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/libs/log"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

const (
	randomSeed int64 = 4827085738
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := NewCLI()
	if err != nil {
		stdlog.Fatal(err)
	}

	cli.Run(ctx)
}

// CLI is the Cobra-based command-line interface.
type CLI struct {
	root   *cobra.Command
	opts   Options
	logger log.Logger
}

// NewCLI sets up the CLI.
func NewCLI() (*CLI, error) {
	logger, err := log.NewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo)
	if err != nil {
		return nil, err
	}

	cli := &CLI{
		logger: logger,
	}
	cli.root = &cobra.Command{
		Use:           "generator",
		Short:         "End-to-end testnet generator",
		SilenceUsage:  true,
		SilenceErrors: true, // we'll output them ourselves in Run()
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.generate()
		},
	}

	cli.root.PersistentFlags().StringVarP(&cli.opts.Directory, "dir", "d", "", "Output directory for manifests")
	_ = cli.root.MarkPersistentFlagRequired("dir")
	cli.root.Flags().BoolVarP(&cli.opts.Reverse, "reverse", "r", false, "Reverse sort order")
	cli.root.PersistentFlags().IntVarP(&cli.opts.NumGroups, "groups", "g", 0, "Number of groups")
	cli.root.PersistentFlags().IntVarP(&cli.opts.MinNetworkSize, "min-size", "", 1,
		"Minimum network size (nodes)")
	cli.root.PersistentFlags().IntVarP(&cli.opts.MaxNetworkSize, "max-size", "", 0,
		"Maxmum network size (nodes), 0 is unlimited")

	return cli, nil
}

// generate generates manifests in a directory.
func (cli *CLI) generate() error {
	err := os.MkdirAll(cli.opts.Directory, 0755)
	if err != nil {
		return err
	}

	manifests, err := Generate(rand.New(rand.NewSource(randomSeed)), cli.opts)
	if err != nil {
		return err
	}

	switch {
	case cli.opts.NumGroups <= 0:
		e2e.SortManifests(manifests, cli.opts.Reverse)

		if err := e2e.WriteManifests(filepath.Join(cli.opts.Directory, "gen"), manifests); err != nil {
			return err
		}
	default:
		groupManifests := e2e.SplitGroups(cli.opts.NumGroups, manifests)

		for idx, gm := range groupManifests {
			e2e.SortManifests(gm, cli.opts.Reverse)

			prefix := filepath.Join(cli.opts.Directory, fmt.Sprintf("gen-group%02d", idx))
			if err := e2e.WriteManifests(prefix, gm); err != nil {
				return err
			}
		}
	}

	return nil
}

// Run runs the CLI.
func (cli *CLI) Run(ctx context.Context) {
	if err := cli.root.ExecuteContext(ctx); err != nil {
		cli.logger.Error(err.Error())
		os.Exit(1)
	}
}
