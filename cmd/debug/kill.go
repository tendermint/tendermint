package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

var (
	nodeAddr string
)

var killCmd = &cobra.Command{
	Use:   "kill [pid] [compressed-output-file]",
	Short: "Kill a Tendermint process while aggregating and packaging debugging data",
	Args:  cobra.ExactArgs(2),
	RunE:  killTendermintProc,
}

func init() {
	// killCmd.Flags().SortFlags = true

	// killCmd.Flags().UintVarP(&pid, "pid", "p", 0, "The Tendermint process PID")
	// killCmd.Flags().StringVarP(&dirOut, "out", "o", 0, "The Tendermint process PID")
	killCmd.Flags().StringVar(&nodeAddr, "node-addr", "tcp://localhost:26657", "The Tendermint RPC address (<host>:<port>)")
}

func killTendermintProc(cmd *cobra.Command, args []string) error {
	// 1. fetch state from /status
	// 2. fetch state from /net_info
	// 3. fetch state from /dump_consensus_state
	// ...

	_, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return err
	}

	// if _, err := os.Stat(outDir); os.IsNotExist(err) {
	// 	if err := os.Mkdir(outDir, os.ModePerm); err != nil {
	// 		return errors.Wrap(err, "failed to create missing directory")
	// 	}
	// }

	// outDir = path.Join(outDir, "tendermint_debug_state")
	// if err := os.Mkdir(outDir, os.ModePerm); err != nil {
	// 	return err
	// }

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

	return zipContentsToFile(tmpDir, args[1])
}

func dumpStatus(rpc *rpcclient.HTTP, dir, filename string) error {
	status, err := rpc.Status()
	if err != nil {
		return errors.Wrap(err, "failed to get node status")
	}

	return writeStateToFile(status, dir, filename)
}

func dumpNetInfo(rpc *rpcclient.HTTP, dir, filename string) error {
	netInfo, err := rpc.NetInfo()
	if err != nil {
		return errors.Wrap(err, "failed to get node network information")
	}

	return writeStateToFile(netInfo, dir, filename)
}

func dumpConsensusState(rpc *rpcclient.HTTP, dir, filename string) error {
	consDump, err := rpc.DumpConsensusState()
	if err != nil {
		return errors.Wrap(err, "failed to get node consensus dump")
	}

	return writeStateToFile(consDump, dir, filename)
}

func writeStateToFile(state interface{}, dir, filename string) error {
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to encode state dump")
	}

	return ioutil.WriteFile(path.Join(dir, filename), stateJSON, os.ModePerm)
}

func zipContentsToFile(src, dest string) error {
	zipFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	baseDir := fmt.Sprintf("tendermint_debug_%d", time.Now().Unix())

	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

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
