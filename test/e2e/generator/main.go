//nolint: gosec
package main

import (
	"fmt"
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

var logger = log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)

func main() {
	NewCLI().Run()
}

// CLI is the Cobra-based command-line interface.
type CLI struct {
	root *cobra.Command
	opts Options
}

// NewCLI sets up the CLI.
func NewCLI() *CLI {
	cli := &CLI{}
	cli.root = &cobra.Command{
		Use:           "generator",
		Short:         "End-to-end testnet generator",
		SilenceUsage:  true,
		SilenceErrors: true, // we'll output them ourselves in Run()
		RunE: func(cmd *cobra.Command, args []string) error {
			var opts Options
			var err error

			cli.opts.Directory, err = cmd.Flags().GetString("dir")
			if err != nil {
				return err
			}

			cli.opts.NumGroups, err = cmd.Flags().GetInt("groups")
			if err != nil {
				return err
			}

			cli.opts.MinNetworkSize, err = cmd.Flags().GetInt("min-size")
			if err != nil {
				return err
			}

			p2pMode, err := cmd.Flags().GetString("p2p")
			if err != nil {
				return err
			}
			switch mode := P2PMode(p2pMode); mode {
			case NewP2PMode, LegacyP2PMode, HybridP2PMode, MixedP2PMode:
				opts.P2P = mode
			default:
				return fmt.Errorf("p2p mode must be either new, legacy, hybrid or mixed got %s", p2pMode)
			}

			return cli.generate()
		},
	}

	cli.root.PersistentFlags().StringP("dir", "d", "", "Output directory for manifests")
	_ = cli.root.MarkPersistentFlagRequired("dir")
	cli.root.PersistentFlags().IntP("groups", "g", 0, "Number of groups")
	cli.root.PersistentFlags().StringP("p2p", "p", string(MixedP2PMode),
		"P2P typology to be generated [\"new\", \"legacy\", \"hybrid\" or \"mixed\" ]")
	cli.root.PersistentFlags().IntP("min-size", "m", 1, "Minimum Network Size")

	return cli
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
		e2e.SortManifests(manifests)

		if err := e2e.WriteManifests(filepath.Join(cli.opts.Directory, "gen"), manifests); err != nil {
			return err
		}
	default:
		groupManifests := e2e.SplitGroups(cli.opts.NumGroups, manifests)

		for idx, gm := range groupManifests {
			e2e.SortManifests(gm)

			prefix := filepath.Join(cli.opts.Directory, fmt.Sprintf("gen-group%02d", idx))
			if err := e2e.WriteManifests(prefix, gm); err != nil {
				return err
			}
		}
	}

	return nil
}

// Run runs the CLI.
func (cli *CLI) Run() {
	if err := cli.root.Execute(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
