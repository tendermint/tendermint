package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/cli"
	tmos "github.com/tendermint/tendermint/libs/os"
)

// clearConfig clears env vars, the given root dir, and resets viper.
func clearConfig(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.Unsetenv("TMHOME"))
	require.NoError(t, os.Unsetenv("TM_HOME"))
	require.NoError(t, os.RemoveAll(dir))

	viper.Reset()
	config = cfg.DefaultConfig()
}

// prepare new rootCmd
func testRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               RootCmd.Use,
		PersistentPreRunE: RootCmd.PersistentPreRunE,
		Run:               func(cmd *cobra.Command, args []string) {},
	}
	registerFlagsRootCmd(rootCmd)
	var l string
	rootCmd.PersistentFlags().String("log", l, "Log")
	return rootCmd
}

func testSetup(t *testing.T, rootDir string, args []string, env map[string]string) error {
	t.Helper()
	clearConfig(t, rootDir)

	rootCmd := testRootCmd()
	cmd := cli.PrepareBaseCmd(rootCmd, "TM", rootDir)

	// run with the args and env
	args = append([]string{rootCmd.Use}, args...)
	return cli.RunWithArgs(cmd, args, env)
}

func TestRootHome(t *testing.T) {
	defaultRoot := t.TempDir()
	newRoot := filepath.Join(defaultRoot, "something-else")
	cases := []struct {
		args []string
		env  map[string]string
		root string
	}{
		{nil, nil, defaultRoot},
		{[]string{"--home", newRoot}, nil, newRoot},
		{nil, map[string]string{"TMHOME": newRoot}, newRoot},
	}

	for i, tc := range cases {
		idxString := strconv.Itoa(i)

		err := testSetup(t, defaultRoot, tc.args, tc.env)
		require.NoError(t, err, idxString)

		assert.Equal(t, tc.root, config.RootDir, idxString)
		assert.Equal(t, tc.root, config.P2P.RootDir, idxString)
		assert.Equal(t, tc.root, config.Consensus.RootDir, idxString)
		assert.Equal(t, tc.root, config.Mempool.RootDir, idxString)
	}
}

func TestRootFlagsEnv(t *testing.T) {

	// defaults
	defaults := cfg.DefaultConfig()
	defaultLogLvl := defaults.LogLevel

	cases := []struct {
		args     []string
		env      map[string]string
		logLevel string
	}{
		{[]string{"--log", "debug"}, nil, defaultLogLvl},                 // wrong flag
		{[]string{"--log-level", "debug"}, nil, "debug"},                 // right flag
		{nil, map[string]string{"TM_LOW": "debug"}, defaultLogLvl},       // wrong env flag
		{nil, map[string]string{"MT_LOG_LEVEL": "debug"}, defaultLogLvl}, // wrong env prefix
		{nil, map[string]string{"TM_LOG_LEVEL": "debug"}, "debug"},       // right env
	}

	defaultRoot := t.TempDir()
	for i, tc := range cases {
		idxString := strconv.Itoa(i)

		err := testSetup(t, defaultRoot, tc.args, tc.env)
		require.NoError(t, err, idxString)

		assert.Equal(t, tc.logLevel, config.LogLevel, idxString)
	}
}

func TestRootConfig(t *testing.T) {

	// write non-default config
	nonDefaultLogLvl := "debug"
	cvals := map[string]string{
		"log-level": nonDefaultLogLvl,
	}

	cases := []struct {
		args []string
		env  map[string]string

		logLvl string
	}{
		{nil, nil, nonDefaultLogLvl},                             // should load config
		{[]string{"--log-level=info"}, nil, "info"},              // flag over rides
		{nil, map[string]string{"TM_LOG_LEVEL": "info"}, "info"}, // env over rides
	}

	for i, tc := range cases {
		defaultRoot := t.TempDir()
		idxString := strconv.Itoa(i)
		clearConfig(t, defaultRoot)

		// XXX: path must match cfg.defaultConfigPath
		configFilePath := filepath.Join(defaultRoot, "config")
		err := tmos.EnsureDir(configFilePath, 0700)
		require.NoError(t, err)

		// write the non-defaults to a different path
		// TODO: support writing sub configs so we can test that too
		err = WriteConfigVals(configFilePath, cvals)
		require.NoError(t, err)

		rootCmd := testRootCmd()
		cmd := cli.PrepareBaseCmd(rootCmd, "TM", defaultRoot)

		// run with the args and env
		tc.args = append([]string{rootCmd.Use}, tc.args...)
		err = cli.RunWithArgs(cmd, tc.args, tc.env)
		require.NoError(t, err, idxString)

		assert.Equal(t, tc.logLvl, config.LogLevel, idxString)
	}
}

// WriteConfigVals writes a toml file with the given values.
// It returns an error if writing was impossible.
func WriteConfigVals(dir string, vals map[string]string) error {
	data := ""
	for k, v := range vals {
		data += fmt.Sprintf("%s = \"%s\"\n", k, v)
	}
	cfile := filepath.Join(dir, "config.toml")
	return os.WriteFile(cfile, []byte(data), 0600)
}
