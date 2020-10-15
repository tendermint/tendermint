package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Cleanup removes the Docker Compose containers and testnet directory.
func Cleanup(testnet *e2e.Testnet) error {
	if testnet.Dir == "" {
		return errors.New("no directory set")
	}
	_, err := os.Stat(testnet.Dir)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	logger.Info("Removing Docker containers and networks")
	err = execCompose(testnet.Dir, "stop")
	if err != nil {
		return err
	}

	// On Linux, some local files in the volume will be owned by root since Tendermint
	// runs as root inside the container, so we need to clean them up from within a
	// container running as root too.
	absDir, err := filepath.Abs(testnet.Dir)
	if err != nil {
		return err
	}
	err = execDocker("run", "--entrypoint", "", "-v", fmt.Sprintf("%v:/network", absDir),
		"tendermint/e2e-node", "sh", "-c", "rm -rf /network/*/")
	if err != nil {
		return err
	}

	err = execCompose(testnet.Dir, "down")
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("Removing testnet directory %q", testnet.Dir))
	err = os.RemoveAll(testnet.Dir)
	if err != nil {
		return err
	}
	return nil
}
