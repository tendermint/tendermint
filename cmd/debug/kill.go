package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	cfg "github.com/tendermint/tendermint/config"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

var (
	nodeAddr string
	nodeHome string
)

var killCmd = &cobra.Command{
	Use:   "kill [pid] [compressed-output-file]",
	Short: "Kill a Tendermint process while aggregating and packaging debugging data",
	Args:  cobra.ExactArgs(2),
	RunE:  killTendermintProc,
}

func init() {
	killCmd.Flags().SortFlags = true

	killCmd.Flags().StringVar(
		&nodeAddr,
		"node-addr",
		"tcp://localhost:26657",
		"The Tendermint node's RPC address (<host>:<port>)",
	)
	killCmd.Flags().StringVar(
		&nodeHome,
		"node-home",
		os.ExpandEnv(filepath.Join("$HOME", cfg.DefaultTendermintDir)),
		"The Tendermint node's home directory",
	)
}

func killTendermintProc(cmd *cobra.Command, args []string) error {
	pid, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return err
	}

	outFile := args[1]
	if outFile == "" {
		return errors.New("invalid output file")
	}

	rpc := rpcclient.NewHTTP(nodeAddr, "/websocket")

	conf := cfg.DefaultConfig()
	conf = conf.SetRoot(nodeHome)
	cfg.EnsureRoot(conf.RootDir)

	// Create a temporary directory which will contain all the state dumps and
	// relevant files and directories that will be compressed into a file.
	tmpDir, err := ioutil.TempDir(os.TempDir(), "tendermint_debug_tmp")
	if err != nil {
		return errors.Wrap(err, "failed to create temporary directory")
	}
	defer os.RemoveAll(tmpDir)

	if err := dumpStatus(rpc, tmpDir, "status.json"); err != nil {
		return err
	}

	if err := dumpNetInfo(rpc, tmpDir, "net_info.json"); err != nil {
		return err
	}

	if err := dumpConsensusState(rpc, tmpDir, "consensus_state.json"); err != nil {
		return err
	}

	if err := copyWAL(conf, tmpDir); err != nil {
		return err
	}

	if err := copyConfig(tmpDir); err != nil {
		return err
	}

	if err := killProc(pid, tmpDir); err != nil {
		return err
	}

	return zipDir(tmpDir, outFile)
}

// dumpStatus gets node status state dump from the Tendermint RPC and writes it
// to file. It returns an error upon failure.
func dumpStatus(rpc *rpcclient.HTTP, dir, filename string) error {
	status, err := rpc.Status()
	if err != nil {
		return errors.Wrap(err, "failed to get node status")
	}

	return writeStateJSONToFile(status, dir, filename)
}

// dumpNetInfo gets network information state dump from the Tendermint RPC and
// writes it to file. It returns an error upon failure.
func dumpNetInfo(rpc *rpcclient.HTTP, dir, filename string) error {
	netInfo, err := rpc.NetInfo()
	if err != nil {
		return errors.Wrap(err, "failed to get node network information")
	}

	return writeStateJSONToFile(netInfo, dir, filename)
}

// dumpConsensusState gets consensus state dump from the Tendermint RPC and
// writes it to file. It returns an error upon failure.
func dumpConsensusState(rpc *rpcclient.HTTP, dir, filename string) error {
	consDump, err := rpc.DumpConsensusState()
	if err != nil {
		return errors.Wrap(err, "failed to get node consensus dump")
	}

	return writeStateJSONToFile(consDump, dir, filename)
}

// copyWAL copies the Tendermint node's WAL file. It returns an error if the
// WAL file cannot be read or copied.
func copyWAL(conf *cfg.Config, dir string) error {
	walPath := conf.Consensus.WalFile()
	walFile := filepath.Base(walPath)

	return copyFile(walPath, filepath.Join(dir, walFile))
}

// copyConfig copies the Tendermint node's config file. It returns an error if
// the config file cannot be read or copied.
func copyConfig(dir string) error {
	configFile := "config.toml"
	configPath := filepath.Join(nodeHome, "config", configFile)

	return copyFile(configPath, filepath.Join(dir, configFile))
}

// killProc attempts to kill the Tendermint process with a given PID with an
// ABORT signal which should result in a goroutine stacktrace. The PID's STDERR
// is tailed and piped to a file under the directory dir. An error is returned
// if the output file cannot be created or the tail command cannot be started.
// An error is not returned if any subsequent syscall fails.
func killProc(pid uint64, dir string) error {
	// pipe STDERR output from tailing the Tendermint process to a file
	cmd := exec.Command("tail", "-f", fmt.Sprintf("/proc/%d/fd/2", pid))

	outFile, err := os.Create(filepath.Join(dir, "stacktrace.out"))
	if err != nil {
		return err
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	if err := cmd.Start(); err != nil {
		return err
	}

	// kill the underlying Tendermint process and subsequent tailing process
	go func() {
		// Killing the Tendermint process with the '-ABRT|-6' signal should result
		// in a goroutine stacktrace.
		if err := syscall.Kill(int(pid), syscall.SIGABRT); err != nil {
			fmt.Fprintf(os.Stderr, "failed to kill Tendermint process: %s", err)
		}

		if err := cmd.Process.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to kill Tendermint process output redirection: %s", err)
		}
	}()

	if err := cmd.Wait(); err != nil {
		// only return an error not invoked by a manual kill
		if _, ok := err.(*exec.ExitError); !ok {
			return err
		}
	}

	return nil
}

// writeStateToFile pretty JSON encodes an object and writes it to file composed
// of dir and filename. It returns an error upon failure to encode or write to
// file.
func writeStateJSONToFile(state interface{}, dir, filename string) error {
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to encode state dump")
	}

	return ioutil.WriteFile(path.Join(dir, filename), stateJSON, os.ModePerm)
}
