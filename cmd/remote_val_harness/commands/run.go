package commands

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/privval"
)

const (
	defaultConnDeadline   = 3
	defaultAcceptDeadline = 1e6
)

var (
	flagAddr           string
	flagKeyFile        string
	flagStateFile      string
	flagGenesisFile    string
	flagConnDeadline   int
	flagAcceptDeadline int
)

func NewRunCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Runs the test harness",
		Long: `
Runs the test harness. Binds to the address indicated by the "addr" parameter,
where this could either be a TCP socket (e.g. tcp://127.0.0.1:10101), or a Unix
socket (e.g. unix:///path/to/socket/file). This will then wait for an incoming
connection from the remote signing service (e.g. KMS:
https://github.com/tendermint/kms), and once this service connects, this
harness' tests will be executed.

Note that using a TCP port of "0" will automatically choose a random open port.`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg := privval.TestHarnessConfig{
				BindAddr:       flagAddr,
				KeyFile:        flagKeyFile,
				StateFile:      flagStateFile,
				GenesisFile:    flagGenesisFile,
				AcceptDeadline: time.Duration(flagAcceptDeadline) * time.Second,
				ConnDeadline:   time.Duration(flagConnDeadline) * time.Second,
				SecretConnKey:  ed25519.GenPrivKey(),
			}
			harness, err := privval.NewTestHarness(logger, cfg)
			if err != nil {
				logger.Error(err.Error())
				if therr, ok := err.(*privval.TestHarnessError); ok {
					os.Exit(therr.Code)
				}
				os.Exit(privval.ErrOther)
			}
			harness.Run()
		},
	}

	runCmd.PersistentFlags().StringVarP(&flagAddr, "addr", "a", "tcp://127.0.0.1:0", "bind to this address, where port 0 will choose a random port")
	runCmd.PersistentFlags().StringVar(&flagKeyFile, "key-file", "~/.tendermint/config/priv_validator_key.json", "path to Tendermint private validator key file")
	runCmd.PersistentFlags().StringVar(&flagStateFile, "state-file", "~/.tendermint/data/priv_validator_state.json", "path to Tendermint private validator state file")
	runCmd.PersistentFlags().StringVar(&flagGenesisFile, "genesis-file", "~/.tendermint/config/genesis.json", "path to Tendermint genesis file")
	runCmd.PersistentFlags().IntVar(&flagAcceptDeadline, "accept-deadline", defaultAcceptDeadline, "accept deadline for connection in seconds")
	runCmd.PersistentFlags().IntVar(&flagConnDeadline, "conn-deadline", defaultConnDeadline, "connect deadline for connection in seconds")

	return runCmd
}
