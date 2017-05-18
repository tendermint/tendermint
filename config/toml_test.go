package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ensureFiles(t *testing.T, rootDir string, files ...string) {
	for _, f := range files {
		p := rootify(rootDir, f)
		_, err := os.Stat(p)
		assert.Nil(t, err, p)
	}
}

func TestEnsureRoot(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// setup temp dir for test
	tmpDir, err := ioutil.TempDir("", "config-test")
	require.Nil(err)
	defer os.RemoveAll(tmpDir)

	// create root dir
	EnsureRoot(tmpDir)

	// make sure config is set properly
	data, err := ioutil.ReadFile(filepath.Join(tmpDir, "config.toml"))
	require.Nil(err)
	assert.Equal([]byte(defaultConfig("anonymous")), data)

	ensureFiles(t, tmpDir, "data")
}

func TestEnsureTestRoot(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	testName := "ensureTestRoot"

	// create root dir
	cfg := ResetTestRoot(testName)
	rootDir := cfg.RootDir

	// make sure config is set properly
	data, err := ioutil.ReadFile(filepath.Join(rootDir, "config.toml"))
	require.Nil(err)
	assert.Equal([]byte(testConfig("anonymous")), data)

	// TODO: make sure the cfg returned and testconfig are the same!

	ensureFiles(t, rootDir, "data", "genesis.json", "priv_validator.json")
}
