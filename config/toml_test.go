package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/test"
)

func ensureFiles(t *testing.T, rootDir string, files ...string) {
	for _, f := range files {
		p := filepath.Join(rootDir, f)
		_, err := os.Stat(p)
		assert.NoError(t, err, p)
	}
}

func TestEnsureRoot(t *testing.T) {
	require := require.New(t)

	// setup temp dir for test
	tmpDir, err := os.MkdirTemp("", "config-test")
	require.Nil(err)
	defer os.RemoveAll(tmpDir)

	// create root dir
	config.EnsureRoot(tmpDir)

	// make sure config is set properly
	data, err := os.ReadFile(filepath.Join(tmpDir, config.DefaultConfigDir, config.DefaultConfigFileName))
	require.Nil(err)

	assertValidConfig(t, string(data))

	ensureFiles(t, tmpDir, "data")
}

func TestEnsureTestRoot(t *testing.T) {
	require := require.New(t)

	// create root dir
	cfg := test.ResetTestRoot("ensureTestRoot")
	defer os.RemoveAll(cfg.RootDir)
	rootDir := cfg.RootDir

	// make sure config is set properly
	data, err := os.ReadFile(filepath.Join(rootDir, config.DefaultConfigDir, config.DefaultConfigFileName))
	require.Nil(err)

	assertValidConfig(t, string(data))

	// TODO: make sure the cfg returned and testconfig are the same!
	baseConfig := config.DefaultBaseConfig()
	ensureFiles(t, rootDir, config.DefaultDataDir, baseConfig.Genesis, baseConfig.PrivValidatorKey, baseConfig.PrivValidatorState)
}

func assertValidConfig(t *testing.T, configFile string) {
	t.Helper()
	// list of words we expect in the config
	var elems = []string{
		"moniker",
		"seeds",
		"proxy_app",
		"block_sync",
		"create_empty_blocks",
		"peer",
		"timeout",
		"broadcast",
		"send",
		"addr",
		"wal",
		"propose",
		"max",
		"genesis",
	}
	for _, e := range elems {
		assert.Contains(t, configFile, e)
	}
}
