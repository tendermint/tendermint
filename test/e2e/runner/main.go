package main

import (
	"context"
	"fmt"
	stdlog "log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/libs/log"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/test/e2e/pkg/infra"
	"github.com/tendermint/tendermint/test/e2e/pkg/infra/docker"
)

const randomSeed = 2308084734268

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, err := log.NewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo)
	if err != nil {
		stdlog.Fatal(err)
	}

	NewCLI(logger).Run(ctx, logger)
}

// CLI is the Cobra-based command-line interface.
type CLI struct {
	root     *cobra.Command
	testnet  *e2e.Testnet
	infra    infra.TestnetInfra
	preserve bool
}

// NewCLI sets up the CLI.
func NewCLI(logger log.Logger) *CLI {
	cli := &CLI{}
	cli.root = &cobra.Command{
		Use:           "runner",
		Short:         "End-to-end test runner",
		SilenceUsage:  true,
		SilenceErrors: true, // we'll output them ourselves in Run()
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			file, err := cmd.Flags().GetString("file")
			if err != nil {
				return err
			}
			testnet, err := e2e.LoadTestnet(file)
			if err != nil {
				return err
			}
			providerID, err := cmd.Flags().GetString("provider")
			if err != nil {
				return err
			}
			switch providerID {
			case "docker":
				cli.infra = docker.NewTestnetInfra(logger, testnet)
				logger.Info("Using Docker-based infrastructure provider")
			default:
				return fmt.Errorf("unrecognized infrastructure provider ID: %s", providerID)
			}

			cli.testnet = testnet
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if err = Cleanup(cmd.Context(), logger, cli.testnet.Dir, cli.infra); err != nil {
				return err
			}
			defer func() {
				if cli.preserve {
					logger.Info("Preserving testnet contents because -preserve=true")
				} else if err != nil {
					logger.Info("Preserving testnet that encountered error",
						"err", err)
				} else if err := Cleanup(cmd.Context(), logger, cli.testnet.Dir, cli.infra); err != nil {
					logger.Error("error cleaning up testnet contents", "err", err)
				}
			}()
			if err = Setup(cmd.Context(), logger, cli.testnet, cli.infra); err != nil {
				return err
			}

			r := rand.New(rand.NewSource(randomSeed)) // nolint: gosec

			chLoadResult := make(chan error)
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			lctx, loadCancel := context.WithCancel(ctx)
			defer loadCancel()
			go func() {
				chLoadResult <- Load(lctx, logger, r, cli.testnet)
			}()
			startAt := time.Now()
			if err = Start(ctx, logger, cli.testnet, cli.infra); err != nil {
				return err
			}

			if err = Wait(ctx, logger, cli.testnet, 5); err != nil { // allow some txs to go through
				return err
			}

			if cli.testnet.HasPerturbations() {
				if err = Perturb(ctx, logger, cli.testnet, cli.infra); err != nil {
					return err
				}
				if err = Wait(ctx, logger, cli.testnet, 5); err != nil { // allow some txs to go through
					return err
				}
			}

			if cli.testnet.Evidence > 0 {
				if err = InjectEvidence(ctx, logger, r, cli.testnet, cli.testnet.Evidence); err != nil {
					return err
				}
				if err = Wait(ctx, logger, cli.testnet, 5); err != nil { // ensure chain progress
					return err
				}
			}

			// to help make sure that we don't run into
			// situations where 0 transactions have
			// happened on quick cases, we make sure that
			// it's been at least 10s before canceling the
			// load generator.
			//
			// TODO allow the load generator to report
			// successful transactions to avoid needing
			// this sleep.
			if rest := time.Since(startAt); rest < 15*time.Second {
				time.Sleep(15*time.Second - rest)
			}

			loadCancel()

			if err = <-chLoadResult; err != nil {
				return fmt.Errorf("transaction load failed: %w", err)
			}
			if err = Wait(ctx, logger, cli.testnet, 5); err != nil { // wait for network to settle before tests
				return err
			}
			if err := Test(ctx, cli.testnet); err != nil {
				return err
			}
			return nil
		},
	}

	cli.root.PersistentFlags().StringP("file", "f", "", "Testnet TOML manifest")
	_ = cli.root.MarkPersistentFlagRequired("file")

	cli.root.PersistentFlags().String("provider", "docker", "Which infrastructure provider to use")

	cli.root.Flags().BoolVarP(&cli.preserve, "preserve", "p", false,
		"Preserves the running of the test net after tests are completed")

	cli.root.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Generates the testnet directory and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Setup(cmd.Context(), logger, cli.testnet, cli.infra)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Starts the Docker testnet, waiting for nodes to become available",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stat(cli.testnet.Dir)
			if os.IsNotExist(err) {
				err = Setup(cmd.Context(), logger, cli.testnet, cli.infra)
			}
			if err != nil {
				return err
			}
			return Start(cmd.Context(), logger, cli.testnet, cli.infra)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "perturb",
		Short: "Perturbs the Docker testnet, e.g. by restarting or disconnecting nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Perturb(cmd.Context(), logger, cli.testnet, cli.infra)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "wait",
		Short: "Waits for a few blocks to be produced and all nodes to catch up",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Wait(cmd.Context(), logger, cli.testnet, 5)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stops the Docker testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Stopping testnet")
			return cli.infra.Stop(cmd.Context())
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "pause",
		Short: "Pauses the Docker testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Pausing testnet")
			return cli.infra.Pause(cmd.Context())
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "resume",
		Short: "Resumes the Docker testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Resuming testnet")
			return cli.infra.Unpause(cmd.Context())
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "load",
		Short: "Generates transaction load until the command is canceled",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return Load(
				cmd.Context(),
				logger,
				rand.New(rand.NewSource(randomSeed)), // nolint: gosec
				cli.testnet,
			)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "evidence [amount]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generates and broadcasts evidence to a random node",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			amount := 1

			if len(args) == 1 {
				amount, err = strconv.Atoi(args[0])
				if err != nil {
					return err
				}
			}

			return InjectEvidence(
				cmd.Context(),
				logger,
				rand.New(rand.NewSource(randomSeed)), // nolint: gosec
				cli.testnet,
				amount,
			)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "test",
		Short: "Runs test cases against a running testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Test(cmd.Context(), cli.testnet)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "cleanup",
		Short: "Removes the testnet directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Cleanup(cmd.Context(), logger, cli.testnet.Dir, cli.infra)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:     "logs [node]",
		Short:   "Shows the testnet or a specific node's logs",
		Example: "runner logs validator03",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				node := cli.testnet.LookupNode(args[0])
				if node == nil {
					return fmt.Errorf("no such node: %s", args[0])
				}
				return cli.infra.ShowNodeLogs(cmd.Context(), node)
			}
			return cli.infra.ShowLogs(cmd.Context())
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "tail [node]",
		Short: "Tails the testnet or a specific node's logs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				node := cli.testnet.LookupNode(args[0])
				if node == nil {
					return fmt.Errorf("no such node: %s", args[0])
				}
				return cli.infra.TailNodeLogs(cmd.Context(), node)
			}
			return cli.infra.TailLogs(cmd.Context())
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "benchmark",
		Short: "Benchmarks testnet",
		Long: `Benchmarks the following metrics:
	Mean Block Interval
	Standard Deviation
	Min Block Interval
	Max Block Interval
over a 100 block sampling period.

Does not run any perbutations.
		`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := Cleanup(cmd.Context(), logger, cli.testnet.Dir, cli.infra); err != nil {
				return err
			}
			defer func() {
				if err := Cleanup(cmd.Context(), logger, cli.testnet.Dir, cli.infra); err != nil {
					logger.Error("error cleaning up testnet contents", "err", err)
				}
			}()

			if err := Setup(cmd.Context(), logger, cli.testnet, cli.infra); err != nil {
				return err
			}

			chLoadResult := make(chan error)
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			r := rand.New(rand.NewSource(randomSeed)) // nolint: gosec

			lctx, loadCancel := context.WithCancel(ctx)
			defer loadCancel()
			go func() {
				chLoadResult <- Load(lctx, logger, r, cli.testnet)
			}()

			if err := Start(ctx, logger, cli.testnet, cli.infra); err != nil {
				return err
			}

			if err := Wait(ctx, logger, cli.testnet, 5); err != nil { // allow some txs to go through
				return err
			}

			// we benchmark performance over the next 100 blocks
			if err := Benchmark(ctx, logger, cli.testnet, 100); err != nil {
				return err
			}

			loadCancel()
			if err := <-chLoadResult; err != nil {
				return err
			}

			return nil
		},
	})

	return cli
}

// Run runs the CLI.
func (cli *CLI) Run(ctx context.Context, logger log.Logger) {
	if err := cli.root.ExecuteContext(ctx); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
