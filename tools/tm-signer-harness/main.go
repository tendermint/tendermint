package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/tools/tm-signer-harness/internal"
	"github.com/tendermint/tendermint/version"
)

const (
	defaultAcceptRetries    = 100
	defaultBindAddr         = "tcp://127.0.0.1:0"
	defaultTMHome           = "~/.tendermint"
	defaultAcceptDeadline   = 1
	defaultConnDeadline     = 3
	defaultExtractKeyOutput = "./signing.key"
)

var logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))

func usage() string {
	var b strings.Builder
	b.WriteString("Remote signer test harness for Tendermint.\n\n")
	b.WriteString("Usage:\n")
	b.WriteString("  tm-signer-harness <command> [args]\n\n")
	b.WriteString("Available Commands:\n")
	b.WriteString("  extract_key        Extracts a signing key from a local Tendermint instance\n")
	b.WriteString("  help               Help on the available commands\n")
	b.WriteString("  run                Runs the test harness\n")
	b.WriteString("  version            Display version information and exit\n\n")
	b.WriteString("Use \"tm-signer-harness help <command>\" for more information on that command.\n")
	return b.String()
}

func cmdUsage(cmd string) string {
	var b strings.Builder
	switch cmd {
	case "run":
		b.WriteString("Runs the remote signer test harness for Tendermint.\n\n")
		b.WriteString("Usage:\n")
		b.WriteString("  tm-signer-harness run [args]\n\n")
		b.WriteString("Flags:\n")
		b.WriteString(
			fmt.Sprintf(
				"  -accept-retries <int>   The number of attempts to listen for incoming connections (default: %d)\n",
				defaultAcceptRetries,
			),
		)
		b.WriteString(
			fmt.Sprintf(
				"  -addr <string>          Bind to this address for the testing (default: \"%s\")\n",
				defaultBindAddr,
			),
		)
		b.WriteString(
			fmt.Sprintf(
				"  -tmhome <string>        Path to the Tendermint home directory (default: \"%s\")\n",
				defaultTMHome,
			),
		)
	case "extract_key":
		b.WriteString("Extracts a signing key from a local Tendermint instance for use in the remote signer under test.\n\n")
		b.WriteString("Usage:\n")
		b.WriteString("  tm-signer-harness extract_key [args]\n\n")
		b.WriteString("Flags:\n")
		b.WriteString(
			fmt.Sprintf(
				"  -output <string>        Path to which the signing key should be written (default: \"%s\")\n",
				defaultExtractKeyOutput,
			),
		)
		b.WriteString(
			fmt.Sprintf(
				"  -tmhome <string>        Path to the Tendermint home directory (default: \"%s\")\n",
				defaultTMHome,
			),
		)
	default:
		fmt.Printf("Unrecognised command: %s\n", cmd)
		os.Exit(1)
	}
	return b.String()
}

func runTestHarness(acceptRetries int, bindAddr, tmhome string) {
	tmhome = internal.ExpandPath(tmhome)
	cfg := internal.TestHarnessConfig{
		BindAddr:         bindAddr,
		KeyFile:          filepath.Join(tmhome, "config", "priv_validator_key.json"),
		StateFile:        filepath.Join(tmhome, "data", "priv_validator_state.json"),
		GenesisFile:      filepath.Join(tmhome, "config", "genesis.json"),
		AcceptDeadline:   time.Duration(defaultAcceptDeadline) * time.Second,
		AcceptRetries:    acceptRetries,
		ConnDeadline:     time.Duration(defaultConnDeadline) * time.Second,
		SecretConnKey:    ed25519.GenPrivKey(),
		ExitWhenComplete: true,
	}
	harness, err := internal.NewTestHarness(logger, cfg)
	if err != nil {
		logger.Error(err.Error())
		if therr, ok := err.(*internal.TestHarnessError); ok {
			os.Exit(therr.Code)
		}
		os.Exit(internal.ErrOther)
	}
	harness.Run()
}

func extractKey(tmhome, outputPath string) {
	keyFile := filepath.Join(internal.ExpandPath(tmhome), "config", "priv_validator_key.json")
	stateFile := filepath.Join(internal.ExpandPath(tmhome), "data", "priv_validator_state.json")
	fpv := privval.LoadFilePV(keyFile, stateFile)
	pkb := [64]byte(fpv.Key.PrivKey.(ed25519.PrivKeyEd25519))
	if err := ioutil.WriteFile(internal.ExpandPath(outputPath), pkb[:32], 0644); err != nil {
		logger.Info("Failed to write private key", "output", outputPath, "err", err)
		os.Exit(1)
	}
	logger.Info("Successfully wrote private key", "output", outputPath)
}

func main() {
	if len(os.Args) == 1 {
		fmt.Println(usage())
		os.Exit(0)
	}

	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	flagRunAcceptRetries := runCmd.Int("accept-retries", defaultAcceptRetries, "")
	flagRunBindAddr := runCmd.String("addr", defaultBindAddr, "")
	flagRunTMHome := runCmd.String("tmhome", defaultTMHome, "")

	extractKeyCmd := flag.NewFlagSet("extract_key", flag.ExitOnError)
	flagExtractKeyTMHome := extractKeyCmd.String("tmhome", defaultTMHome, "")
	flagExtractKeyOutputPath := extractKeyCmd.String("output", defaultExtractKeyOutput, "")

	logger = log.NewFilter(logger, log.AllowInfo())

	switch os.Args[1] {
	case "help":
		if len(os.Args) > 2 {
			fmt.Println(cmdUsage(os.Args[2]))
		} else {
			fmt.Println(usage())
		}
	case "run":
		runCmd.Parse(os.Args[2:])
		runTestHarness(*flagRunAcceptRetries, *flagRunBindAddr, *flagRunTMHome)
	case "extract_key":
		extractKeyCmd.Parse(os.Args[2:])
		extractKey(*flagExtractKeyTMHome, *flagExtractKeyOutputPath)
	case "version":
		fmt.Println(version.Version)
	default:
		fmt.Printf("Unrecognised command: %s", os.Args[1])
		os.Exit(1)
	}
}
