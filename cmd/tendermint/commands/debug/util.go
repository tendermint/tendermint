package debug

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"

	cfg "github.com/tendermint/tendermint/config"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
)

// dumpStatus gets node status state dump from the Tendermint RPC and writes it
// to file. It returns an error upon failure.
func dumpStatus(rpc *rpchttp.HTTP, dir, filename string) error {
	status, err := rpc.Status()
	if err != nil {
		return errors.Wrap(err, "failed to get node status")
	}

	return writeStateJSONToFile(status, dir, filename)
}

// dumpNetInfo gets network information state dump from the Tendermint RPC and
// writes it to file. It returns an error upon failure.
func dumpNetInfo(rpc *rpchttp.HTTP, dir, filename string) error {
	netInfo, err := rpc.NetInfo()
	if err != nil {
		return errors.Wrap(err, "failed to get node network information")
	}

	return writeStateJSONToFile(netInfo, dir, filename)
}

// dumpConsensusState gets consensus state dump from the Tendermint RPC and
// writes it to file. It returns an error upon failure.
func dumpConsensusState(rpc *rpchttp.HTTP, dir, filename string) error {
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
func copyConfig(home, dir string) error {
	configFile := "config.toml"
	configPath := filepath.Join(home, "config", configFile)

	return copyFile(configPath, filepath.Join(dir, configFile))
}

func dumpProfile(dir, addr, profile string, debug int) error {
	endpoint := fmt.Sprintf("%s/debug/pprof/%s?debug=%d", addr, profile, debug)

	resp, err := http.Get(endpoint) // nolint: gosec
	if err != nil {
		return errors.Wrapf(err, "failed to query for %s profile", profile)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s profile response body", profile)
	}

	return ioutil.WriteFile(path.Join(dir, fmt.Sprintf("%s.out", profile)), body, os.ModePerm)
}
