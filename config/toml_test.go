package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ensureFiles(t *testing.T, rootDir string, files ...string) {
	for _, f := range files {
		p := rootify(rootDir, f)
		_, err := os.Stat(p)
		assert.NoError(t, err, p)
	}
}

func TestEnsureRoot(t *testing.T) {
	// setup temp dir for test
	tmpDir := t.TempDir()

	// create root dir
	EnsureRoot(tmpDir)

	require.NoError(t, WriteConfigFile(tmpDir, DefaultConfig()))

	// make sure config is set properly
	data, err := os.ReadFile(filepath.Join(tmpDir, defaultConfigFilePath))
	require.NoError(t, err)

	checkConfig(t, string(data))

	ensureFiles(t, tmpDir, "data")
}

func TestEnsureTestRoot(t *testing.T) {
	testName := "ensureTestRoot"

	// create root dir
	cfg, err := ResetTestRoot(t.TempDir(), testName)
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	rootDir := cfg.RootDir

	// make sure config is set properly
	data, err := os.ReadFile(filepath.Join(rootDir, defaultConfigFilePath))
	require.NoError(t, err)

	checkConfig(t, string(data))

	// TODO: make sure the cfg returned and testconfig are the same!
	baseConfig := DefaultBaseConfig()
	pvConfig := DefaultPrivValidatorConfig()
	ensureFiles(t, rootDir, defaultDataDir, baseConfig.Genesis, pvConfig.Key, pvConfig.State)
}

func checkConfig(t *testing.T, configFile string) {
	t.Helper()
	// list of words we expect in the config
	var elems = []string{
		"moniker",
		"seeds",
		"proxy-app",
		"create-empty-blocks",
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
		if !strings.Contains(configFile, e) {
			t.Errorf("config file was expected to contain %s but did not", e)
		}
	}
}
