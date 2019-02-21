package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

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
	// 4. config
	// 5. WAL

	_, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return err
	}

	rpc := rpcclient.NewHTTP(nodeAddr, "/websocket")

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

	return zipDir(tmpDir, args[1])
}

// dumpStatus gets node status state dump from the Tendermint RPC and writes it
// to file. It returns an error upon failure.
func dumpStatus(rpc *rpcclient.HTTP, dir, filename string) error {
	status, err := rpc.Status()
	if err != nil {
		return errors.Wrap(err, "failed to get node status")
	}

	return writeStateToFile(status, dir, filename)
}

// dumpNetInfo gets network information state dump from the Tendermint RPC and
// writes it to file. It returns an error upon failure.
func dumpNetInfo(rpc *rpcclient.HTTP, dir, filename string) error {
	netInfo, err := rpc.NetInfo()
	if err != nil {
		return errors.Wrap(err, "failed to get node network information")
	}

	return writeStateToFile(netInfo, dir, filename)
}

// dumpConsensusState gets consensus state dump from the Tendermint RPC and
// writes it to file. It returns an error upon failure.
func dumpConsensusState(rpc *rpcclient.HTTP, dir, filename string) error {
	consDump, err := rpc.DumpConsensusState()
	if err != nil {
		return errors.Wrap(err, "failed to get node consensus dump")
	}

	return writeStateToFile(consDump, dir, filename)
}

// writeStateToFile pretty JSON encodes an object and writes it to file composed
// of dir and filename. It returns an error upon failure to encode or write to
// file.
func writeStateToFile(state interface{}, dir, filename string) error {
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to encode state dump")
	}

	return ioutil.WriteFile(path.Join(dir, filename), stateJSON, os.ModePerm)
}

// zipDir zips all the contents found in src, including both files and
// directories, into a destination file dest. It returns an error upon failure.
// It assumes src is a directory.
func zipDir(src, dest string) error {
	zipFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	dirName := filepath.Base(dest)
	baseDir := strings.TrimSuffix(dirName, filepath.Ext(dirName))

	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Each execution of this utility on a Tendermint process will result in a
		// unique file.
		header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, src))

		// Handle cases where the content to be zipped is a file or a directory,
		// where a directory must have a '/' suffix.
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		headerWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(headerWriter, file)
		return err
	})

	return nil
}
