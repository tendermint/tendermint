package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/libs/log"
)

const (
	randomSeed int64 = 4827085738
)

var logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))

func main() {
	NewCLI().Run()
}

// CLI is the Cobra-based command-line interface.
type CLI struct {
	root *cobra.Command
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
			dir, err := cmd.Flags().GetString("dir")
			if err != nil {
				return err
			}
			groups, err := cmd.Flags().GetInt("groups")
			if err != nil {
				return err
			}
			return cli.generate(dir, groups)
		},
	}

	cli.root.PersistentFlags().StringP("dir", "d", "", "Output directory for manifests")
	_ = cli.root.MarkPersistentFlagRequired("dir")
	cli.root.PersistentFlags().IntP("groups", "g", 0, "Number of groups")

	return cli
}

// generate generates manifests in a directory.
func (cli *CLI) generate(dir string, groups int) error {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}

	//nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand)
	manifests, err := Generate(rand.New(rand.NewSource(randomSeed)))
	if err != nil {
		return err
	}
	if groups <= 0 {
		for i, manifest := range manifests {
			err = manifest.Save(filepath.Join(dir, fmt.Sprintf("gen-%04d.toml", i)))
			if err != nil {
				return err
			}
		}
	} else {
		groupSize := int(math.Ceil(float64(len(manifests)) / float64(groups)))
		for g := 0; g < groups; g++ {
			for i := 0; i < groupSize && g*groupSize+i < len(manifests); i++ {
				manifest := manifests[g*groupSize+i]
				err = manifest.Save(filepath.Join(dir, fmt.Sprintf("gen-group%02d-%04d.toml", g, i)))
				if err != nil {
					return err
				}
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
